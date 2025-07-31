package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// GetTempDir returns the appropriate temporary directory for the current OS
func GetTempDir() string {
	if runtime.GOOS == "windows" {
		return os.TempDir()
	}
	return "/tmp"
}

// GetProjectRoot returns the project root directory by finding go.mod
func GetProjectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Go up directories until we find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("Could not find project root (go.mod not found)")
	return ""
}

// BuildTempBinary builds the speakeasy CLI to a temporary location and returns the path
func BuildTempBinary(t *testing.T, binaryPath string) string {
	// Delete the binary if it exists
	os.Remove(binaryPath)

	// Build the speakeasy CLI
	t.Logf("Building speakeasy CLI to: %s", binaryPath)
	projectRoot := GetProjectRoot(t)
	t.Logf("Project root directory: %s", projectRoot)

	// Build from the project root
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()
	t.Logf("Build output: %s", string(buildOutput))
	require.NoError(t, err, "Failed to build speakeasy CLI: %s", string(buildOutput))

	// Check it exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Speakeasy binary not found at %s", binaryPath)
	}

	// Make the binary executable (only on Unix-like systems)
	if runtime.GOOS != "windows" {
		err = os.Chmod(binaryPath, 0755)
		require.NoError(t, err, "Failed to make binary executable")
	}

	// Check version
	versionCmd := exec.Command(binaryPath, "--version")
	versionOutput, err := versionCmd.CombinedOutput()
	t.Logf("Version output: %s", string(versionOutput))
	require.NoError(t, err, "Failed to get speakeasy version")

	return binaryPath
}
