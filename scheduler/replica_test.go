package main

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

func TestStartStopReplica(t *testing.T) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        t.Error(err)
    }

    ctx := context.Background()

    rep, err := NewReplica(cli, ctx)
    if err != nil {
        t.Error(err)
    }
    print(rep.ID)
    err = rep.Cleanup(cli, ctx)
    if err != nil {
        t.Error(err)
    }
}
