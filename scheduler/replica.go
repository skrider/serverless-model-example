package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Replica struct {
	ID   string
	Name string
	IP   string
}

type ReplicaRequest struct {
	input string
}

type ReplicaResponse struct {
	output string
}

func NewReplica(cli *client.Client, ctx context.Context, index int) (*Replica, error) {
	replicaImage := os.Getenv("APP_REPLICA_IMAGE")
	replicaNetworkName := "serverless-model-example_default"
	if replicaImage == "" {
		panic("APP_REPLICA_IMAGE environment variable not set")
	}

	containerName := fmt.Sprintf("replica_%d", index)

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

	containerResponse, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return nil, err
	}

	err = cli.ContainerStart(ctx, containerResponse.ID, types.ContainerStartOptions{})
	if err != nil {
		cli.ContainerRemove(ctx, containerResponse.ID, types.ContainerRemoveOptions{})
		return nil, err
	}

	containerInfo, err := cli.ContainerInspect(ctx, containerResponse.ID)
	if err != nil {
		return nil, err
	}

	containerIP := containerInfo.NetworkSettings.Networks[replicaNetworkName].IPAddress
	replica := &Replica{
		ID:   containerResponse.ID,
		Name: containerName,
		IP:   containerIP,
	}

	return replica, err
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

func (r *Replica) Predict(input string) (string, error) {
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
