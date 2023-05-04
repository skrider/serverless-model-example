package main

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func TestStartStopReplica(t *testing.T) {
	t.Skip()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	rep := NewReplica()

	rep.SetStatus(ReplicaStarting)
	err = rep.Setup(cli, ctx)
	if err != nil {
		t.Error(err)
		rep.SetStatus(ReplicaError)
	}

	rep.SetStatus(ReplicaStopped)
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
		rep.SetStatus(ReplicaError)
	}

	err = rep.ok()
	if err == nil {
		t.Error("rep is still ok")
	}

	rep.SetStatus(ReplicaTerminated)
}

func TestReplicaPredict(t *testing.T) {
	t.Skip()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	job := &Job{
		Input:    "hello",
		ID:       "1",
		Output:   "",
		Status:   JobPending,
		Duration: time.Second * 2,
	}

	rep := NewReplica()

	rep.SetStatus(ReplicaStarting)
	err = rep.Setup(cli, ctx)
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}

	rep.EnqueueJob(job)

	rep.SetStatus(ReplicaRunning)
	err = rep.Run()
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}

	if job.Output == "" {
		t.Error("job output is empty")
	}

	rep.SetStatus(ReplicaStopped)
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}

	rep.SetStatus(ReplicaTerminated)

	err = rep.ok()
	if err == nil {
		t.Error("rep is still ok")
	}
}

func TestReplicaPredictAsync(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	job1 := &Job{
		Input:    "hello",
		ID:       "1",
		Output:   "",
		Status:   JobPending,
		Duration: time.Second * 2,
	}
	job2 := &Job{
		Input:    "hello 2",
		ID:       "2",
		Output:   "",
		Status:   JobPending,
		Duration: time.Second * 2,
	}

	rep := NewReplica()

	rep.SetStatus(ReplicaStarting)
	err = rep.Setup(cli, ctx)
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}
	rep.EnqueueJob(job1)

	go func() {
		time.Sleep(time.Second)
		rep.EnqueueJob(job2)
	}()

	rep.SetStatus(ReplicaRunning)
	err = rep.Run()
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}

	if job1.Output == "" {
		t.Error("job 1 output is empty")
	}
	if job2.Output == "" {
		t.Error("job 2 output is empty")
	}

	rep.SetStatus(ReplicaStopped)
	err = rep.Cleanup(cli, ctx)
	if err != nil {
		rep.SetStatus(ReplicaError)
		panic(err)
	}

	rep.SetStatus(ReplicaTerminated)

	err = rep.ok()
	if err == nil {
		t.Error("rep is still ok")
	}
}
