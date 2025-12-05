package integration_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// prebuiltBinary holds the path to the pre-built speakeasy binary.
// This avoids recompiling on every `go run main.go` call, saving ~20s per invocation.
var prebuiltBinary string

// Entrypoint for CLI integration tests
func TestMain(m *testing.M) {
	// Create a temporary directory
	if _, err := os.Stat(tempDir); err == nil {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}

	if err := os.Mkdir(tempDir, 0o755); err != nil {
		panic(err)
	}

	// Pre-build the speakeasy binary once to avoid ~20s compilation overhead per test
	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	binaryName := "speakeasy-test-binary"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(os.TempDir(), binaryName)

	fmt.Println("Pre-building speakeasy binary for integration tests...")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(baseFolder, "main.go"))
	buildCmd.Dir = baseFolder
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		panic(fmt.Sprintf("failed to pre-build speakeasy binary: %v", err))
	}
	prebuiltBinary = binaryPath
	fmt.Println("Pre-built speakeasy binary:", prebuiltBinary)

	// Defer the removal of the temp directory and binary
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
		os.Remove(prebuiltBinary)
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
