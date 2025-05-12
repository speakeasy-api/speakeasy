package locks

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

// CLIUpdateMutex provides file-based mutual exclusion between processes.
// The lock is automatically released if the holding process dies.
//
// See:
//   - Linux: https://linux.die.net/man/2/flock
//   - Windows: https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
type CLIUpdateMutex struct {
	Opts
	mu *flock.Flock
}

type Opts struct {
	Name string
}

func DefaultOpts() Opts {
	return Opts{Name: "speakeasy.lock"}
}

func new(o Opts) *CLIUpdateMutex {
	mu := flock.New(filepath.Join(os.TempDir(), o.Name))

	return &CLIUpdateMutex{Opts: o, mu: mu}
}

// CLIUpdateLockWithOpts creates a new inter-process mutex with a custom config.
var CLIUpdateLockWithOpts = singleton.NewWithOpts(func(o Opts) *CLIUpdateMutex {
	return new(o)
})

// CLIUpdateLock creates a new inter-process mutex with default options.
var CLIUpdateLock = singleton.New(func() *CLIUpdateMutex {
	return new(DefaultOpts())
})

type TryLockResult struct {
	Attempt int
	Error   error
	Success bool
}

func (m *CLIUpdateMutex) TryLock(ctx context.Context, retryDelay time.Duration) <-chan TryLockResult {
	ch := make(chan TryLockResult)
	go func() {
		for attempt := 0; ; attempt++ {
			ok, err := m.mu.TryLock()
			if err != nil {
				ch <- TryLockResult{Attempt: attempt, Error: fmt.Errorf("failed to acquire lock (pid %d): %w", os.Getpid(), err)}
				return
			}
			if ok {
				ch <- TryLockResult{Attempt: attempt, Success: true}
				return
			}

			select {
			case <-ctx.Done():
				ch <- TryLockResult{Attempt: attempt, Error: ctx.Err()}
				return
			case <-time.After(retryDelay):
				ch <- TryLockResult{Attempt: attempt, Success: false}
			}
		}
	}()
	return ch
}

func (m *CLIUpdateMutex) Unlock() error {
	return m.mu.Unlock()
}
