package main

import (
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
	Duration time.Duration
	start    time.Time
	end      time.Time
}

func (j *Job) GetStatusString() string {
	switch j.Status {
	case JobPending:
		return "queued"
	case JobRunning:
		return "processing"
	case JobDone:
		return "finished"
	}
	return "error"
}

func (j *Job) Start() {
	j.start = time.Now()
	j.Status = JobRunning
}

func (j *Job) Finish() {
	j.end = time.Now()
	j.Status = JobDone
}

func (j *Job) Latency() time.Duration {
	return j.end.Sub(j.start)
}