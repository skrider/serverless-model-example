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

	err = rep.Setup(cli, ctx)
	if err != nil {
		t.Error(err)
	}

	err = rep.Cleanup(cli, ctx)
	if err != nil {
		t.Error(err)
	}

	err = rep.ok()
	if err == nil {
		t.Error("rep is still ok")
	}
    if rep.Status() != ReplicaTerminated {
        t.Error("rep is not terminated")
    }
}

func TestReplicaPredict(t *testing.T) {
	t.Skip()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	job := MakeJob("test 1")
	rep := NewReplica()

	err = rep.Setup(cli, ctx)
	if err != nil {
		panic(err)
	}

	rep.EnqueueJob(job)

	err = rep.Run()
	if err != nil {
		panic(err)
	}

	if job.Output == "" {
		t.Error("job output is empty")
	}
	if job.Status != JobDone {
		t.Error("job status is not finished")
	}

	err = rep.Cleanup(cli, ctx)
	if err != nil {
		panic(err)
	}

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

    job1 := MakeJob("test 3")
	job2 := MakeJob("test 4")

	rep := NewReplica()

	err = rep.Setup(cli, ctx)
	if err != nil {
		rep.setStatus(ReplicaError)
		panic(err)
	}
	rep.EnqueueJob(job1)

	go func() {
		time.Sleep(time.Second)
		rep.EnqueueJob(job2)
	}()

	err = rep.Run()
	if err != nil {
		panic(err)
	}

	if job1.Output == "" {
		t.Error("job 1 output is empty")
	}
	if job2.Output == "" {
		t.Error("job 2 output is empty")
	}

	err = rep.Cleanup(cli, ctx)
	if err != nil {
		panic(err)
	}

	err = rep.ok()
	if err == nil {
		t.Error("rep is still ok")
	}
}
