package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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
	id                string
	name              string
	ip                string
	setupDuration     time.Duration
	timeStarted       time.Time
	jobQueue          chan *Job
	cumulativeJobTime time.Duration
	mu                sync.Mutex
	workOnce          sync.Once
	status            ReplicaStatus
}

type ReplicaStatus int

const (
	ReplicaInit ReplicaStatus = iota
	ReplicaRunning
	ReplicaStopped
	ReplicaTerminated
	ReplicaError
)

var replicaNetworkName string
var replicaImage string

var replicaNum int
var replicaNumMutex sync.Mutex

const defaultSetupTime = 4 * time.Second

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
		name:              fmt.Sprintf("replica_%d", replicaNum),
		jobQueue:          make(chan *Job, 100),
		cumulativeJobTime: 0,
		status:            ReplicaInit,
		setupDuration:     defaultSetupTime,
	}
	replicaNum += 1

	return replica
}

func (r *Replica) TimeToReady() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cumulativeJobTime + r.setupDuration - time.Since(r.timeStarted)
}

func (r *Replica) Status() ReplicaStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Replica) EnqueueJob(job *Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobQueue <- job
	r.cumulativeJobTime += job.Duration
	job.Status = JobPending
}

func (r *Replica) BeginWork(cli *client.Client, ctx context.Context) error {
	r.mu.Lock()
    if r.status != ReplicaInit {
        return errors.New("replica already working")
    }
	r.timeStarted = time.Now()
	r.status = ReplicaRunning
	r.mu.Unlock()

	err := r.setup(cli, ctx)
	if err != nil {
		r.mu.Lock()
		r.status = ReplicaError
		r.mu.Unlock()
        return err
	}

	for {
		r.mu.Lock()
		if len(r.jobQueue) == 0 {
			break
		}
		job := <-r.jobQueue
		r.mu.Unlock()

		job.Status = JobRunning

		job.Output, err = r.predict(job.Input)

		if err != nil {
			job.Status = JobError
		}

		job.Status = JobDone
	}
	r.status = ReplicaStopped
	r.mu.Unlock()

	err = r.cleanup(cli, ctx)
    if err != nil {
        return err
    }
    r.mu.Lock()
    r.status = ReplicaTerminated
    r.mu.Unlock()

    return nil
}

func (r *Replica) setup(cli *client.Client, ctx context.Context) error {
	r.timeStarted = time.Now()
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

	containerResponse, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, r.name)
	if err != nil {
		return err
	}

	err = cli.ContainerStart(ctx, containerResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		cli.ContainerRemove(ctx, containerResponse.ID, types.ContainerRemoveOptions{})
		return err
	}

	containerInfo, err := cli.ContainerInspect(ctx, containerResponse.ID)
	if err != nil {
		return err
	}

	r.ip = containerInfo.NetworkSettings.Networks[replicaNetworkName].IPAddress
	r.id = containerResponse.ID

	for i := 0; i < 100; i++ {
		err := r.ok()
		time.Sleep(500 * time.Millisecond)
		if err == nil {
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
	r.timeStarted = time.Now()
	endpoint := fmt.Sprintf("http://%s:8000", r.ip)
	reqBody := fmt.Sprintf(`{"input": "%s"}`, input)

	response, err := http.Post(endpoint, "application/json", strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (r *Replica) cleanup(cli *client.Client, ctx context.Context) error {
	stopOptions := container.StopOptions{}

	err := cli.ContainerStop(ctx, r.id, stopOptions)
	if err != nil {
		return err
	}

	err = cli.ContainerRemove(ctx, r.id, types.ContainerRemoveOptions{})
	return err
}
