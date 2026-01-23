package integration_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// prebuiltBinary holds the path to the pre-built speakeasy binary.
// This avoids recompiling on every `go run main.go` call, saving ~20s per invocation.
var (
	prebuiltBinary string
	buildOnce      sync.Once
	errBuild       error
)

// ensureBinary builds the speakeasy binary once on first call.
// Subsequent calls return immediately. This is called lazily by execute()
// so that executeI() invocations don't pay the build cost.
func ensureBinary() (string, error) {
	buildOnce.Do(func() {
		_, filename, _, _ := runtime.Caller(0)
		baseFolder := filepath.Join(filepath.Dir(filename), "..")
		// Use PID to avoid collision between parallel test runs on the same machine
		binaryName := fmt.Sprintf("speakeasy-test-binary-%d", os.Getpid())
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
			errBuild = fmt.Errorf("failed to pre-build speakeasy binary: %w", err)
			return
		}
		prebuiltBinary = binaryPath
		fmt.Println("Pre-built speakeasy binary:", prebuiltBinary)
	})
	return prebuiltBinary, errBuild
}

// Entrypoint for CLI integration tests
func TestMain(m *testing.M) {
	testDir := integrationTestsDir()

	// Create the integrationTests directory (MkdirAll is safe for parallel test processes)
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		panic(err)
	}

	code := m.Run()

	// Cleanup must happen before os.Exit (defer is not executed with os.Exit)
	if prebuiltBinary != "" {
		os.Remove(prebuiltBinary)
	}

	os.Exit(code)
}
