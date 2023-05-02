package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func TestStartStopReplica(t *testing.T) {
	t.Parallel()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	rep, err := NewReplica(cli, ctx, 1)
	if err != nil {
		t.Error(err)
	}
	println(rep.ID)
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
	}
}

func TestReplicaOk(t *testing.T) {
	t.Parallel()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	rep, err := NewReplica(cli, ctx, 2)
	if err != nil {
		t.Error(err)
	}

	for true {
		ok, _ := rep.Ok()
		time.Sleep(500 * time.Millisecond)
		if ok {
			break
		}
	}
	println("container ok")

	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
	}
}

func TestReplicaPredict(t *testing.T) {
	t.Parallel()
	startTime := time.Now()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	println("creating replica", time.Since(startTime).Milliseconds())

	rep, err := NewReplica(cli, ctx, 3)
	if err != nil {
		t.Error(err)
	}

	println("waiting for ok", time.Since(startTime).Milliseconds())
	for true {
		ok, _ := rep.Ok()
		time.Sleep(500 * time.Millisecond)
		if ok {
			break
		}
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
