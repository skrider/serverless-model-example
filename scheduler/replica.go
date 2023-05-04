package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Replica struct {
	id            string        // docker ID
	Name          string        // docker name
	ip            string        // docker IP, used for communicating
	timeWhenReady time.Time     // estimated time to ready
	jobQueue      chan *Job     // queue of jobs to be processed
	mu            sync.Mutex    // mutex for accessing and modifying replica state
	doWorkOnce    sync.Once     // once to ensure BeginWork is only called once
	status        ReplicaStatus // current status of replica
}

type ReplicaStatus int

const (
	ReplicaInit       ReplicaStatus = iota // replica object has been created, however there is no container running
	ReplicaStarting                        // replica container is running
	ReplicaRunning                         // replica container is running
	ReplicaStopped                         // replica container has been stopped but not fully cleaned up
	ReplicaTerminated                      // replica has been fully cleaned up
	ReplicaError                           // replica is in an error state
)

var replicaNetworkName string
var replicaImage string

var replicaNum int
var replicaNumMutex sync.Mutex

func init() {
	replicaImage = os.Getenv("APP_REPLICA_IMAGE")

	replicaNetworkName = os.Getenv("APP_REPLICA_NETWORK")
	if replicaNetworkName == "" {
		replicaNetworkName = "serverless-model-example_default"
	}

	replicaNum = 0
}

func NewReplica() *Replica {
	replicaNumMutex.Lock()
	defer replicaNumMutex.Unlock()

	replica := &Replica{
		Name:     fmt.Sprintf("replica_%d", replicaNum),
		jobQueue: make(chan *Job, 100),
		status:   ReplicaInit,
	}
	replicaNum += 1

	return replica
}

func (r *Replica) TimeToReady() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == ReplicaTerminated || r.status == ReplicaStopped || r.status == ReplicaInit {
		return time.Duration(1<<63 - 1)
	}
	return time.Until(r.timeWhenReady)
}

func (r *Replica) Status() ReplicaStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Replica) setStatus(status ReplicaStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	log.Printf("[%s] status changed from %s to %s", r.Name, r.status.String(), status.String())
	r.status = status
}

func (r *Replica) EnqueueJob(job *Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == ReplicaStopped || r.status == ReplicaTerminated || r.status == ReplicaError {
		return errors.New(fmt.Sprintf("replica %s is in bad state %s", r.Name, r.status.String()))
	}
	r.jobQueue <- job
	job.Status = JobPending
	r.timeWhenReady = r.timeWhenReady.Add(job.Duration)
	return nil
}

func (r *Replica) Run() error {
	r.setStatus(ReplicaRunning)
	var err error
	for {
		if len(r.jobQueue) == 0 {
			return nil
		}
		job := <-r.jobQueue

		job.Output, err = r.predict(job.Input)
		if err != nil {
			job.Status = JobError
			return err
		}

		job.Finish()
	}
}

func (r *Replica) Setup(cli *client.Client, ctx context.Context) error {
	r.mu.Lock()
	r.timeWhenReady = time.Now().Add(SetupDuration.GetTime())
	r.mu.Unlock()
	r.setStatus(ReplicaStarting)

	startTime := time.Now()

	containerConfig := &container.Config{
		Image: replicaImage,
		ExposedPorts: nat.PortSet{
			"8000/tcp": struct{}{},
		},
	}
	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			replicaNetworkName: {},
		},
	}

	log.Printf("[%s] creating replica container", r.Name)
	containerResponse, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, r.Name)
	if err != nil {
		r.setStatus(ReplicaError)
		return err
	}

	log.Printf("[%s] starting replica container", r.Name)
	err = cli.ContainerStart(ctx, containerResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		cli.ContainerRemove(ctx, containerResponse.ID, types.ContainerRemoveOptions{})
		r.setStatus(ReplicaError)
		return err
	}

	log.Printf("[%s] resolving address", r.Name)
	containerInfo, err := cli.ContainerInspect(ctx, containerResponse.ID)
	if err != nil {
		r.setStatus(ReplicaError)
		return err
	}

	r.ip = containerInfo.NetworkSettings.Networks[replicaNetworkName].IPAddress
	r.id = containerResponse.ID

	log.Printf("[%s] pinging (waiting for model setup)", r.Name)
	for i := 0; i < 100; i++ {
		err := r.ok()
		time.Sleep(500 * time.Millisecond)
		if err == nil {
			r.mu.Lock()
			r.timeWhenReady = time.Now()
			SetupDuration.UpdateTime(time.Since(startTime))
			r.mu.Unlock()
			return nil
		}
	}

	return errors.New("replica setup failed")
}

func (r *Replica) ok() error {
	endpoint := fmt.Sprintf("http://%s:8000/ok", r.ip)
	response, err := http.Get(endpoint)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if string(body) == "ok" {
		return nil
	}

	return errors.New("malformed response from replica")
}

func (r *Replica) predict(input string) (string, error) {
	endpoint := fmt.Sprintf("http://%s:8000", r.ip)
	reqBody := fmt.Sprintf(`{"input": "%s"}`, input)

	startTime := time.Now()
	response, err := http.Post(endpoint, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	JobDuration.UpdateTime(time.Since(startTime))

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// parse body as json
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}

	return data["output"].(string), nil
}

func (r *Replica) Cleanup(cli *client.Client, ctx context.Context) error {
	r.setStatus(ReplicaStopped)
	log.Printf("[%s] stopping replica container", r.Name)
	stopOptions := container.StopOptions{}

	err := cli.ContainerStop(ctx, r.id, stopOptions)
	if err != nil {
		r.setStatus(ReplicaError)
		return err
	}

	log.Printf("[%s] removing replica container", r.Name)
	err = cli.ContainerRemove(ctx, r.id, types.ContainerRemoveOptions{})
	if err != nil {
		r.setStatus(ReplicaError)
		return err
	}

	r.setStatus(ReplicaTerminated)
	return nil
}

func (s ReplicaStatus) String() string {
	switch s {
	case ReplicaInit:
		return "init"
	case ReplicaStarting:
		return "starting"
	case ReplicaRunning:
		return "running"
	case ReplicaStopped:
		return "stopped"
	case ReplicaTerminated:
		return "terminated"
	}
	return "error"
}
