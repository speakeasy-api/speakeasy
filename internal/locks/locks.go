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

// Package locks provides utilities for inter-process synchronization.

// InterProcessMutex provides file-based mutual exclusion between processes.
// It uses file locking to ensure that only one process can perform CLI updates at a time.
// The lock is automatically released if the holding process dies.
//
// This implementation is cross-platform compatible:
//   - Linux/macOS: Uses flock syscall (https://linux.die.net/man/2/flock)
//   - Windows: Uses LockFileEx API (https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
//
// The mutex can be used to coordinate CLI update operations between multiple
// concurrent processes to prevent race conditions during installation or updates.
type InterProcessMutex struct {
	Opts
	mu *flock.Flock
}

type Opts struct {
	Name string
}

func new(o Opts) *InterProcessMutex {
	mu := flock.New(filepath.Join(os.TempDir(), o.Name))

	return &InterProcessMutex{Opts: o, mu: mu}
}

// CLIUpdateLock creates a new inter-process mutex with default options.
var CLIUpdateLock = singleton.New(func() *InterProcessMutex {
	return new(Opts{Name: "speakeasy-cli-update-mutex.lock"})
})

type TryLockResult struct {
	Attempt int
	Error   error
	Success bool
}

func (m *InterProcessMutex) TryLock(ctx context.Context, retryDelay time.Duration) <-chan TryLockResult {
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

func (m *InterProcessMutex) Unlock() error {
	return m.mu.Unlock()
}
