package main

import (
	"sync"
	"time"
)

type JobStatus int

const (
	JobPending JobStatus = iota
	JobRunning
	JobDone
	JobError
)

type Job struct {
	Input    string
	ID       string
	Output   string
	Status   JobStatus
	mu       sync.Mutex
	Duration time.Duration
}
