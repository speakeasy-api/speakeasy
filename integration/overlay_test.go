package integration_tests

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOverlayMatchesSnapshot(t *testing.T) {
	t.Parallel()
	_, filename, _, _ := runtime.Caller(0)
	overlayFolder := filepath.Join(filepath.Dir(filename), "resources", "overlay")
	overlayFilePath, err := filepath.Abs(filepath.Join(overlayFolder, "overlay.yaml"))
	require.NoError(t, err)
	schemaFilePath, err := filepath.Abs(filepath.Join(overlayFolder, "openapi.yaml"))
	require.NoError(t, err)
	expectedBytes, err := os.ReadFile(filepath.Join(overlayFolder, "openapi-overlayed-expected.yaml"))
	require.NoError(t, err)

	temp := setupTestDir(t)
	outputPath, err := filepath.Abs(filepath.Join(temp, "output.yaml"))
	require.NoError(t, err)

	args := []string{"overlay", "apply", "--schema", schemaFilePath, "--overlay", overlayFilePath, "--out", outputPath}
	cmdErr := execute(t, temp, args...).Run()
	require.NoError(t, cmdErr)
	readBytes, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	// Normalize line endings for both expected and actual content
	expectedNormalized := normalizeLineEndings(string(expectedBytes))
	actualNormalized := normalizeLineEndings(string(readBytes))

	require.Equal(t, expectedNormalized, actualNormalized)
}

// normalizeLineEndings replaces all occurrences of \r\n with \n
func normalizeLineEndings(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
