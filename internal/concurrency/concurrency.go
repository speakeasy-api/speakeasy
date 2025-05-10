package concurrency

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/speakeasy-api/speakeasy/internal/singleton"
)

// Package concurrency provides utilities for inter-process synchronization .

type InterProcessMutex struct {
	Opts
	mu *flock.Flock
}

type Opts struct {
	Name           string
	LockRetryDelay time.Duration
}

func DefaultOpts() Opts {
	return Opts{Name: "speakeasy-lock", LockRetryDelay: 10 * time.Second}
}

func new(o Opts) *InterProcessMutex {
	mu := flock.New(filepath.Join(os.TempDir(), o.Name))

	return &InterProcessMutex{mu: mu}
}

// NewIPMutexWithOpts creates a new inter-process mutex with a custom config.
var NewIPMutexWithOpts = singleton.NewWithOpts(func(o Opts) *InterProcessMutex {
	return new(o)
})

// NewIPMutex creates a new inter-process mutex with the default config.
var NewIPMutex = singleton.New(func() *InterProcessMutex {
	return new(DefaultOpts())
})

func (m *InterProcessMutex) Lock() error {
	return m.mu.Lock()
}

func (m *InterProcessMutex) Unlock() error {
	return m.mu.Unlock()
}

func (m *InterProcessMutex) TryLock(ctx context.Context) (bool, error) {
	return m.mu.TryLockContext(ctx, m.LockRetryDelay)
}
