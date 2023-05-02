package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

type State struct {
	replicas  []*Replica
	jobs      []*Job
	jobTime   time.Duration
	setupTime time.Duration
	mu        sync.Mutex
}

type JobStatus int

const (
	JobPending JobStatus = iota
	JobRunning
	JobDone
	JobError
)

type Job struct {
	Input  string
	ID     string
	Output string
	Status JobStatus
}

func (s *State) AddJob(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timeToReady := s.computeMinTimeToReady()

	minTime := time.Duration(0)
	for i := range s.replicas {
		if timeToReady[i] > minTime {
			minTime = timeToReady[i]
		}
	}

	if minTime > s.setupTime {
        s.AddReplica()
	}

}

func (s *State) AddReplica() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	replica, err := NewReplica(cli, ctx, len(s.replicas))
	if err != nil {
		panic(err)
	}

	s.replicas = append(s.replicas, replica)
}

func (s *State) GetJob(id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range s.jobs {
		if job.ID == id {
			return job, nil
		}
	}

	return nil, errors.New("Job not found")
}

func (s *State) computeMinTimeToReady() []time.Duration {
	minTime := make([]time.Duration, len(s.replicas))

	for i, replica := range s.replicas {
		minTime[i] = replica.TimeToReady()
	}

	for _, job := range s.jobs {
		if job.Status == JobPending {
			argmin := 0
			for j, _ := range s.replicas {
				if minTime[j] < minTime[argmin] {
					argmin = j
				}
			}
			minTime[argmin] += s.jobTime
		}
	}
	return minTime
}
