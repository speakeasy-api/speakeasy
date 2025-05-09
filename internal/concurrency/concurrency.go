package concurrency

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/config"
)

// ErrLockExists indicates another process already holds the lock
var ErrLockExists = errors.New("lock already exists: another Speakeasy process is running. You can: 1) wait for it to complete, 2) wait 1 minute for the lock to expire, or 3) manually remove the .lock file in your ~/.speakeasy directory if no other Speakeasy process is running")

// MaxLockAge defines how old a lock can be before it's considered stale (1 minute)
const MaxLockAge = time.Minute

// AcquireLock attempts to atomically create a lock file
// Returns nil if successful, ErrLockExists if another process holds the lock
func AcquireLock() error {
	speakeasyHomeDir, err := config.GetSpeakeasyHomeDir()
	if err != nil {
		return err
	}

	lockFile := filepath.Join(speakeasyHomeDir, ".lock")

	// Check if lock exists and is stale
	if _, err := os.Stat(lockFile); err == nil {
		// Lock exists, check if it's stale
		if isStale, _ := isLockStale(lockFile); isStale {
			// Lock is stale, remove it
			os.Remove(lockFile)
		} else {
			return ErrLockExists
		}
	}

	// O_EXCL with O_CREATE ensures the call fails if the file already exists
	// This makes the check-and-create atomic
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return ErrLockExists
		}
		return err
	}

	// Write the current PID and timestamp to help with debugging
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	if _, err := file.WriteString(filepath.Base(os.Args[0]) + "\n" + timestamp + "\n"); err != nil {
		file.Close()
		os.Remove(lockFile)
		return err
	}

	return file.Close()
}

// UpdateLock updates the timestamp in the lock file
func UpdateLock() error {
	speakeasyHomeDir, err := config.GetSpeakeasyHomeDir()
	if err != nil {
		return err
	}

	lockFile := filepath.Join(speakeasyHomeDir, ".lock")

	// Check if we own the lock
	content, err := os.ReadFile(lockFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || lines[0] != filepath.Base(os.Args[0]) {
		return errors.New("lock not owned by this process")
	}

	// Update the lock file with a new timestamp
	file, err := os.OpenFile(lockFile, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	_, err = file.WriteString(filepath.Base(os.Args[0]) + "\n" + timestamp + "\n")
	return err
}

// isLockStale checks if the lock file is older than MaxLockAge
func isLockStale(lockFile string) (bool, error) {
	content, err := os.ReadFile(lockFile)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 {
		// Malformed lock file, consider it stale
		return true, nil
	}

	timestamp, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		// Malformed timestamp, consider it stale
		return true, nil
	}

	lockTime := time.Unix(timestamp, 0)
	return time.Since(lockTime) > MaxLockAge, nil
}

// ReleaseLock removes the lock file
func ReleaseLock() error {
	speakeasyHomeDir, err := config.GetSpeakeasyHomeDir()
	if err != nil {
		return err
	}

	lockFile := filepath.Join(speakeasyHomeDir, ".lock")
	return os.Remove(lockFile)
}

func SafeExit(code int) {
	ReleaseLock()
	os.Exit(code)
}
