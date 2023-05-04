package main

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/client"
)

const DOCKER_OVERHEAD time.Duration = 2 * time.Second // additional time that Docker takes to set up a replica

var replicas []*Replica  // contains all replicas, including deprovisioned replicas
var jobs map[string]*Job // contains all jobs
var jobQueue chan *Job   // channel for passing jobs from request handler goroutines to background job manager
var cli *client.Client   // docker client

var JobDuration *MovingAverageDuration
var SetupDuration *MovingAverageDuration

var timeTolerance time.Duration = 500 * time.Millisecond

func init() {
	var err error
	// populate constants from environment
	// have to do it in init() so they are available during testing
	jobDuration, err := strconv.Atoi(os.Getenv("MODEL_PREDICT_TIME"))
	if err != nil {
		panic(err)
	}
	JobDuration = MakeMovingAverageDuration(time.Duration(jobDuration) * time.Second)

	setupDuration, err := strconv.Atoi(os.Getenv("MODEL_SETUP_TIME"))
	if err != nil {
		panic(err)
	}
	SetupDuration = MakeMovingAverageDuration(time.Duration(setupDuration)*time.Second + DOCKER_OVERHEAD)
}

func main() {
	var err error

	rand.Seed(time.Now().UnixNano())

	jobQueue = make(chan *Job, 100)
	jobs = make(map[string]*Job)
	replicas = make([]*Replica, 0)

	// add a sentinel replica that never gets initialized to make state logic easier
	replicas = append(replicas, NewReplica())

	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// start the background persistence worker for backing up jobs
	go backgroundPersistenceWorker()

	// start the background job worker for managing jobs and replicas
	go backgroundJobWorker()

	log.Print("[main] Listening on port 8000")
	log.Printf("[main] Setup time:    %d", SetupDuration.GetTime())
	log.Printf("[main] Handling time: %d", JobDuration.GetTime())

	http.HandleFunc("/push", pushHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/data/", dataHandler)
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func backgroundJobWorker() {
	for {
		job := <-jobQueue
		log.Printf("[main] Handling job %s", job.ID)

		// find the soonest-available replica
		min := replicas[0].TimeToReady()
		argmin := 0
		for i, replica := range replicas {
			currentTime := replica.TimeToReady()
			if currentTime < min {
				argmin = i
				min = currentTime
			}
		}
		log.Printf("[main] job %s:  min: %d, argmin: %d", job.ID, min, argmin)

		// if the soonest-available replica will take longer to be available then
		// setting up a  new replica, then spin up a new replica
        // add timeTolerance to take into account mismatch between timeToReady and
        // actual time
		if min > SetupDuration.GetTime() - timeTolerance {
			log.Printf("[main] job %s: spinning up a new replica", job.ID)
			rep := NewReplica()
			replicas = append(replicas, rep)
			// start a goroutine to manage the new replica
			go backgroundReplicaWorker(rep)
			err := rep.EnqueueJob(job)
			if err != nil {
				panic(err)
			}
		} else {
			log.Printf("[main] assigning job %s to replica %s", job.ID, replicas[argmin].Name)
			err := replicas[argmin].EnqueueJob(job)
			if err != nil {
				panic(err)
			}
		}
	}

}

func backgroundReplicaWorker(rep *Replica) {
	var err error
	ctx := context.Background()

	// start the replica
	err = rep.Setup(cli, ctx)
	if err != nil {
		rep.setStatus(ReplicaError)
		panic(err)
	}

	// run until the job queue has been exhausted
	err = rep.Run()
	if err != nil {
		panic(err)
	}

	// clean up the replica
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		panic(err)
	}
}

func backgroundPersistenceWorker() {
	// load state from disk
	filepath := os.Getenv("STATE_FILE")
	if filepath == "" {
		log.Printf("[main] No state file specified, not loading state from disk")
		return
	}
	log.Printf("[main] Loading state from disk")
	file, err := os.Open(filepath)
	if err != nil {
		log.Printf("[main] Error opening file: %v\n", err)
	}

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&jobs)
	if err != nil {
		log.Printf("[main] Error decoding map: %v\n", err)
	}

	file.Close()
	for {
		time.Sleep(10 * time.Second)
		log.Printf("[main] Persisting state to disk")
		file, err := os.Create(filepath)
		if err != nil {
			log.Printf("[main] Error creating file: %v\n", err)
			return
		}
		defer file.Close()

		// Create a gob encoder
		encoder := gob.NewEncoder(file)

		// Serialize the map to the file
		err = encoder.Encode(jobs)
		if err != nil {
			log.Printf("[main] Error encoding map: %v\n", err)
			return
		}
	}

}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	// parse job from request
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
	}

	bodyJson := make(map[string]interface{})
	err = json.Unmarshal(body, &bodyJson)
	if err != nil {
		http.Error(w, "Error parsing request body", http.StatusInternalServerError)
	}

	log.Printf("[main] Received job: %v", bodyJson)

	job := MakeJob(bodyJson["input"].(string))
	jobQueue <- job

	jobs[job.ID] = job

	// return job id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": job.ID})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	// parse job from request
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// get job id from slug
	jobID := r.URL.Path[len("/status/"):]

	job, ok := jobs[jobID]
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// return job status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": job.GetStatusString()})
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	// parse job from request
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// get job id from slug
	jobID := r.URL.Path[len("/data/"):]

	job, ok := jobs[jobID]
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	if job.Status != JobDone {
		http.Error(w, "Job not finished", http.StatusNotFound)
		return
	}

	// return job status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"input":   job.Input,
		"output":  job.Output,
		"latency": job.Latency().String(),
	})
}
