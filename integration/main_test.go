package integration_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Entrypoint for CLI integration tests
func TestMain(m *testing.M) {
	os.Setenv("SPEAKEASY_CONCURRENCY_LOCK_DISABLED", "true")

	// Create a temporary directory
	if _, err := os.Stat(tempDir); err == nil {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}

	if err := os.Mkdir(tempDir, 0o755); err != nil {
		panic(err)
	}

	// Defer the removal of the temp directory
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	code := m.Run()
	os.Exit(code)
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	workingDir := filepath.Dir(filename)
	temp, err := createTempDir(workingDir)
	assert.NoError(t, err)
	registerCleanup(t, workingDir, temp)

	return temp
}

func registerCleanup(t *testing.T, workingDir string, temp string) {
	t.Helper()
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(workingDir, temp))
	})
}
