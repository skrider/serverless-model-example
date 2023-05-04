package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

func replicaStateMonitor(rep *Replica) {
    currentStatus := rep.Status()
    for {
        switch currentStatus {
        case ReplicaRunning:
            // print replica is running as well as the current time
            fmt.Printf("[%s] %s running\n", time.Now().Format(time.RFC3339), rep.name)
        case ReplicaStopped:
            fmt.Printf("[%s] %s stopped\n", time.Now().Format(time.RFC3339), rep.name)
        case ReplicaError:
            fmt.Printf("[%s] %s error\n", time.Now().Format(time.RFC3339), rep.name)
        case ReplicaTerminated:
            fmt.Printf("[%s] %s terminated\n", time.Now().Format(time.RFC3339), rep.name)
            return
        }
        prevStatus := currentStatus
        for currentStatus == prevStatus { 
            currentStatus = rep.Status()
        }
    }
}

func TestStartStopReplica(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	rep := NewReplica()

    // state monitor
    go replicaStateMonitor(rep)

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

    go replicaStateMonitor(rep)

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
