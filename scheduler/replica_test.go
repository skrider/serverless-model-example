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
		t.Error(err)
	}
	ctx := context.Background()

	rep := NewReplica()

    // state monitor
    go rep.MonitorReplicaState()

    err = rep.BeginWork(cli, ctx)
    if err != nil {
        t.Error(err)
    }

    err = rep.ok()
    if err == nil {
        t.Error("rep is still ok")
    }
}

func TestReplicaPredict(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()


    job := &Job{
        Input: "hello",
        ID: "1",
        Output: "",
        Status: JobPending,
        Duration: time.Second * 2,
    }

	rep := NewReplica()

    go rep.MonitorReplicaState()

    rep.EnqueueJob(job)

    err = rep.BeginWork(cli, ctx)

    if err != nil {
        t.Error(err)
    }

    if job.Output == "" {
        t.Error("job output is empty")
    }

    err = rep.ok()
    if err == nil {
        t.Error("rep is still ok")
    }
}
