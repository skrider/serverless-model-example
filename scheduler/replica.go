package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type ReplicaStatus int

const (
	ReplicaCreated ReplicaStatus = iota
	ReplicaStarting 
	ReplicaRunning
	ReplicaIdle
	ReplicaError
)

type Replica struct {
	ID          string
	Name        string
	IP          string
	Status      ReplicaStatus
	timeStarted time.Time
}

var ReplicaNetworkName string = "serverless-model-example_default"
var ReplicaImage string = os.Getenv("APP_REPLICA_IMAGE")

type ReplicaRequest struct {
	input string
}

type ReplicaResponse struct {
	output string
}

func NewReplica(index int) *Replica {
	containerName := fmt.Sprintf("replica_%d", index)

	replica := &Replica{
		Name:        containerName,
		Status:      ReplicaCreated,
		timeStarted: time.Now(),
	}

	return replica
}

func (r *Replica) Ok() (bool, error) {
	endpoint := fmt.Sprintf("http://%s:8000/ok", r.IP)
	response, err := http.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, err
	}

	if string(body) == "ok" {
		return true, nil
	}
	return false, nil
}

func (r *Replica) Setup(cli *client.Client, ctx context.Context) error {
    r.timeStarted = time.Now()
	containerConfig := &container.Config{
		Image: ReplicaImage,
		ExposedPorts: nat.PortSet{
			"8000/tcp": struct{}{},
		},
	}
	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			ReplicaNetworkName: {},
		},
	}

	containerResponse, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, r.Name)
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

	r.IP = containerInfo.NetworkSettings.Networks[ReplicaNetworkName].IPAddress
	r.ID = containerResponse.ID

	for i := 0; i < 100; i++ {
		ok, _ := r.Ok()
		time.Sleep(500 * time.Millisecond)
		if ok {
			return nil
		}
	}

	return errors.New("replica setup failed")
}

func (r *Replica) TimeToReady() time.Duration {
	if r.Status == ReplicaIdle {
		return time.Duration(0)
	}
	return time.Now().Sub(r.timeStarted)
}

func (r *Replica) Predict(input string) (string, error) {
    r.timeStarted = time.Now()
	endpoint := fmt.Sprintf("http://%s:8000", r.IP)
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

func (r *Replica) Cleanup(cli *client.Client, ctx context.Context) error {
	stopOptions := container.StopOptions{}

	err := cli.ContainerStop(ctx, r.ID, stopOptions)
	if err != nil {
		return err
	}

	err = cli.ContainerRemove(ctx, r.ID, types.ContainerRemoveOptions{})
	return err
}
