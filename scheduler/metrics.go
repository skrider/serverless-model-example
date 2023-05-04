package main

import (
	"sync"
	"time"
)

type MovingAverageDuration struct {
	mu   sync.Mutex
	time time.Duration
	n    int
}

func MakeMovingAverageDuration(initial time.Duration) *MovingAverageDuration {
	return &MovingAverageDuration{
		time: initial,
		n:    1,
	}
}

func (m *MovingAverageDuration) GetTime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.time
}

func (m *MovingAverageDuration) UpdateTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.time = time.Duration((int(m.time)*m.n + int(d)) / (m.n + 1))
	m.n += 1
}
