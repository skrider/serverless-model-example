package main

import (
	"context"
    "os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type Replica struct {
	ID string
}

type ReplicaRequest struct {
	input string
}

type ReplicaResponse struct {
	output string
}

func NewReplica(cli *client.Client, ctx context.Context) (*Replica, error) {
    workerImage := os.Getenv("APP_WORKER_IMAGE")
    if workerImage == "" {
        panic("APP_WORKER_IMAGE environment variable not set")
    }

	containerConfig := &container.Config{
		Image: workerImage,
	}
	hostConfig := &container.HostConfig{
        NetworkMode: "host",
    }
	networkConfig := &network.NetworkingConfig{}

	containerResponse, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, "")
	if err != nil {
		return nil, err
	}

	err = cli.ContainerStart(ctx, containerResponse.ID, types.ContainerStartOptions{})
    if err != nil {
        return nil, err
    }

    return &Replica{ID: containerResponse.ID}, err
}

func (r *Replica) Predict(req *ReplicaRequest) (*ReplicaResponse, error) {
    return &ReplicaResponse{output: req.input}, nil
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
