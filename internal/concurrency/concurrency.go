package concurrency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/speakeasy-api/speakeasy/internal/singleton"
)

// Package concurrency provides utilities for inter-process synchronization.

// InterProcessMutex provides file-based mutual exclusion between processes.
// The lock is automatically released if the holding process dies.
//
// See:
//   - Linux: https://linux.die.net/man/2/flock
//   - Windows: https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
type InterProcessMutex struct {
	Opts
	mu *flock.Flock
}

type Opts struct {
	Name           string
	LockRetryDelay time.Duration
}

func DefaultOpts() Opts {
	return Opts{Name: "speakeasy.lock", LockRetryDelay: 10 * time.Second}
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

func (m *InterProcessMutex) TryLock(ctx context.Context, onRetry func(attempt int)) error {
	attempt := 0
	for {
		ok, err := m.mu.TryLockContext(ctx, m.LockRetryDelay)
		if err != nil {
			return fmt.Errorf("failed to acquire lock (pid %d): %w", os.Getpid(), err)
		}
		if ok {
			return nil
		}
		attempt++
		if onRetry != nil {
			onRetry(attempt)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.LockRetryDelay):
			continue
		}
	}
}

func (m *InterProcessMutex) Unlock() error {
	return m.mu.Unlock()
}
