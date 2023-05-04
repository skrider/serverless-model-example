package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func TestStartStopReplica(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	rep:= NewReplica(1)
    rep.Setup(cli, ctx)
	if err != nil {
		t.Error(err)
	}
	println(rep.ID)
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
	}
}

func TestReplicaPredict(t *testing.T) {
	startTime := time.Now()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	println("creating replica", time.Since(startTime).Milliseconds())

	rep := NewReplica(2)
    err = rep.Setup(cli, ctx)
	if err != nil {
		t.Error(err)
	}

	println("container ok", time.Since(startTime).Milliseconds())
	println("predicting")
	output, err := rep.Predict("test")
	if err != nil {
		t.Error(err)
	}
	println(output)
	println("predicted", time.Since(startTime).Milliseconds())
	println("cleaning up")
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
	}
	println("done", time.Since(startTime).Milliseconds())
}
