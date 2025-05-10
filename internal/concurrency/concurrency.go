package concurrency

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/speakeasy-api/speakeasy/internal/singleton"
)

// Package concurrency provides utilities for inter-process synchronization and coordination.
// It contains helpers for managing concurrent access to shared resources and communication
// between different processes in a safe and efficient manner.

type InterProcessMutex struct {
	mu *flock.Flock
}

type Config struct {
	Path string
}

func DefaultConfig() Config {
	return Config{Path: filepath.Join(os.TempDir(), "speakeasy-lock")}
}

func new(cfg Config) *InterProcessMutex {
	mu := flock.New(cfg.Path)

	return &InterProcessMutex{mu: mu}
}

// NewIPMutexWithConfig creates a new inter-process mutex with a custom config.
var NewIPMutexWithConfig = singleton.NewWithConfig(func(c Config) *InterProcessMutex {
	return new(c)
})

// NewIPMutex creates a new inter-process mutex with the default config.
var NewIPMutex = singleton.New(func() *InterProcessMutex {
	return new(DefaultConfig())
})

func (m *InterProcessMutex) Lock() error {
	return m.mu.Lock()
}

func (m *InterProcessMutex) Unlock() error {
	return m.mu.Unlock()
}

func (m *InterProcessMutex) TryLock() (bool, error) {
	return m.mu.TryLock()
}
