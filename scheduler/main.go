package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/docker/docker/client"
)

var replicas []*Replica
var jobs map[string]*Job
var globalJobQueue chan *Job

var cli *client.Client


func init() {
}

func main() {
    globalJobQueue = make(chan *Job, 100)
    jobs = make(map[string]*Job)
    replicas = make([]*Replica, 0)

    // add a sentinel replica that never gets used
    replicas = append(replicas, NewReplica())

    var err error
	cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

    go backgroundJobWorker()

	log.Print("[main] Listening 8000")
	log.Printf("[main] Setup time %d", DEFAULT_SETUP_TIME)
	log.Printf("[main] Handling time %d", DEFAULT_JOB_DURATION)

	http.HandleFunc("/push", pushHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/data/", dataHandler)
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func backgroundJobWorker() {
    for {
        job := <- globalJobQueue
        log.Printf("[main] Handling job %s", job.ID)

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
        if min > DEFAULT_SETUP_TIME {
            log.Printf("[main] job %s: spinning up a new replica", job.ID)
            rep := NewReplica()
            replicas = append(replicas, rep)
            go manageReplica(rep)
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

func manageReplica(rep *Replica) {
    go rep.MonitorReplicaState()
    ctx := context.Background()
    err := rep.BeginWork(cli, ctx)
    if err != nil {
        panic(err)
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

	job := &Job{
		ID:       UUID(),
		Input:    bodyJson["input"].(string),
		Status:   JobPending,
		Duration: DEFAULT_JOB_DURATION,
	}

    globalJobQueue <- job

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
        "input": job.Input,
        "output": job.Output,
        "latency": job.Latency().String(),
    })
}

