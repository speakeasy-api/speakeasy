package concurrency

import "github.com/gofrs/flock"

// Package concurrency provides utilities for inter-process synchronization and coordination.
// It contains helpers for managing concurrent access to shared resources and communication
// between different processes in a safe and efficient manner.

type InterProcessMutex struct {
	mu *flock.Flock
}

func New(path string) (*InterProcessMutex, error) {
	mu := flock.New(path)

	return &InterProcessMutex{mu: mu}, nil
}

func (m *InterProcessMutex) Lock() error {
	return m.mu.Lock()
}

func (m *InterProcessMutex) Unlock() error {
	return m.mu.Unlock()
}

func (m *InterProcessMutex) TryLock() (bool, error) {
	return m.mu.TryLock()
}
