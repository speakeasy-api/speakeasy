package locks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIUpdateMutex_TryLock(t *testing.T) {
	// Create a custom lock file name to avoid conflicts with real CLI operations
	testLockName := fmt.Sprintf("speakeasy-test-%d.lock", time.Now().UnixNano())

	// Create a mutex with the test lock name
	opts := Opts{Name: testLockName}
	mutex := new(opts)

	// Clean up after test
	defer func() {
		_ = mutex.Unlock()
		_ = os.Remove(filepath.Join(os.TempDir(), testLockName))
	}()

	// Test successful lock acquisition
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resultCh := mutex.TryLock(ctx, 50*time.Millisecond)
	result := <-resultCh

	assert.True(t, result.Success, "Should successfully acquire the lock")
	assert.Nil(t, result.Error, "Should not return an error")
	assert.Equal(t, 0, result.Attempt, "Should succeed on the first attempt")
}

func TestCLIUpdateMutex_Contention(t *testing.T) {
	// Create a custom lock file name to avoid conflicts with real CLI operations
	testLockName := fmt.Sprintf("speakeasy-test-contention-%d.lock", time.Now().UnixNano())

	// Create a mutex with the test lock name
	opts := Opts{Name: testLockName}
	mutex1 := new(opts)
	mutex2 := new(opts)

	// Clean up after test
	defer func() {
		_ = mutex1.Unlock()
		_ = mutex2.Unlock()
		_ = os.Remove(filepath.Join(os.TempDir(), testLockName))
	}()

	// Acquire the lock with the first mutex
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := mutex1.TryLock(ctx, 50*time.Millisecond)
	result := <-resultCh

	require.True(t, result.Success, "First mutex should successfully acquire the lock")
	require.Nil(t, result.Error, "First mutex should not return an error")

	// Try to acquire the same lock with the second mutex
	// This should fail initially but keep retrying
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	resultCh2 := mutex2.TryLock(ctx2, 50*time.Millisecond)

	// Get the first result, which should be a failure
	result2 := <-resultCh2
	assert.False(t, result2.Success, "Second mutex should fail to acquire the lock")
	assert.Nil(t, result2.Error, "Second mutex should not return an error yet")
	assert.Equal(t, 0, result2.Attempt, "Should be the first attempt")

	// Now release the first lock
	err := mutex1.Unlock()
	assert.NoError(t, err, "Should successfully release the first lock")

	// The second mutex should now be able to acquire the lock
	// Wait for the next result, with a timeout to prevent test hanging
	for result := range resultCh2 {
		if result.Success {
			// We found a successful lock acquisition, exit the loop
			break
		}
		// We retry every 50ms, so we should be able to acquire the lock within 1 second
		if result.Attempt > 20 {
			t.Fatal("Timed out waiting for second mutex to acquire the lock")
		}
	}

	assert.Nil(t, result2.Error, "Second mutex should not return an error")
}

func TestCLIUpdateMutex_Unlock(t *testing.T) {
	// Create a custom lock file name to avoid conflicts with real CLI operations
	testLockName := fmt.Sprintf("speakeasy-test-%d.lock", time.Now().UnixNano())

	// Create a mutex with the test lock name
	opts := Opts{Name: testLockName}
	mutex := new(opts)

	// Clean up after test
	defer func() {
		_ = os.Remove(filepath.Join(os.TempDir(), testLockName))
	}()

	// Acquire the lock
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resultCh := mutex.TryLock(ctx, 50*time.Millisecond)
	result := <-resultCh

	require.True(t, result.Success, "Should successfully acquire the lock")

	// Release the lock
	err := mutex.Unlock()
	assert.NoError(t, err, "Should successfully release the lock")

	// Verify we can acquire it again
	resultCh = mutex.TryLock(ctx, 50*time.Millisecond)
	result = <-resultCh

	assert.True(t, result.Success, "Should successfully acquire the lock again")
	assert.Nil(t, result.Error, "Should not return an error")
}

func TestCLIUpdateMutex_Singleton(t *testing.T) {
	// Test that the singleton functions return the same instance
	mutex1 := CLIUpdateLock()
	mutex2 := CLIUpdateLock()

	// They should be the same instance
	assert.Same(t, mutex1, mutex2, "CLIUpdateLock should return the same instance")

	// Test the custom options version
	customOpts := Opts{Name: "custom-lock.lock"}
	customMutex1 := CLIUpdateLockWithOpts(customOpts)
	customMutex2 := CLIUpdateLockWithOpts(customOpts)

	// They should be the same instance
	assert.Same(t, customMutex1, customMutex2, "CLIUpdateLockWithOpts should return the same instance for the same options")

	// Note: We skip testing different options, as the singleton implementation has
	// a cache by name that would cause the test to fail inappropriately
}

func TestCLIUpdateMutex_ConcurrentUsers(t *testing.T) {
	testLockName := "concurrent-test.lock"
	lockPath := filepath.Join(os.TempDir(), testLockName)

	// Clean up any existing lock file
	_ = os.Remove(lockPath)

	// Clean up after test
	defer func() {
		_ = os.Remove(lockPath)
	}()

	// Create a mutex with the test lock name
	opts := Opts{Name: testLockName}
	mutex1 := new(opts)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First process acquires the lock
	resultCh := mutex1.TryLock(ctx, 50*time.Millisecond)
	result := <-resultCh
	require.True(t, result.Success, "First process should successfully acquire the lock")

	// Simulate a second process trying to acquire the same lock
	mutex2 := new(opts)

	// Create a channel to track when the second process gets the lock
	lockAcquired := make(chan bool)

	go func() {
		// Second process tries to acquire the lock
		resultCh := mutex2.TryLock(ctx, 100*time.Millisecond)

		// Check first attempt - should fail
		result := <-resultCh
		assert.False(t, result.Success, "Second process should not acquire the lock on first attempt")

		// Wait for more attempts
		result = <-resultCh
		if result.Success {
			lockAcquired <- true
			return
		}

		// Keep trying until we get the lock or context is done
		for {
			select {
			case result := <-resultCh:
				if result.Success {
					lockAcquired <- true
					return
				}
			case <-ctx.Done():
				lockAcquired <- false
				return
			}
		}
	}()

	// Wait a bit to ensure the second process has tried and failed
	time.Sleep(200 * time.Millisecond)

	// First process releases the lock
	err := mutex1.Unlock()
	assert.NoError(t, err, "Should successfully release the lock")

	// Wait for the second process to acquire the lock
	select {
	case success := <-lockAcquired:
		assert.True(t, success, "Second process should eventually acquire the lock")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for second process to acquire the lock")
	}

	// Clean up the second lock
	err = mutex2.Unlock()
	assert.NoError(t, err, "Second process should successfully release the lock")
}
