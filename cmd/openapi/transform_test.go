package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunFormat_ReadableStyle(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.json")
	outputPath := filepath.Join(tempDir, "output.json")
	input := `{"paths":{},"openapi":"3.0.0","info":{"version":"1","title":"T"}}`
	require.NoError(t, os.WriteFile(inputPath, []byte(input), 0o600))

	err := runFormat(context.Background(), formatFlags{
		Schema: inputPath,
		Out:    outputPath,
		Style:  "readable",
	})
	require.NoError(t, err)

	actual, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(actual), "readable formatting should preserve the document")
}

func TestRunFormat_ReadableUppercaseYAMLOutput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.json")
	input := `{"paths":{},"openapi":"3.0.0","info":{"version":"1","title":"T"}}`
	require.NoError(t, os.WriteFile(inputPath, []byte(input), 0o600))

	for _, outputName := range []string{"output.YML", "output.YAML"} {
		outputPath := filepath.Join(tempDir, outputName)
		err := runFormat(context.Background(), formatFlags{
			Schema: inputPath,
			Out:    outputPath,
			Style:  "readable",
		})
		require.NoError(t, err)

		actual, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(actual), "openapi: 3.0.0", "uppercase YAML extensions should emit YAML")
	}
}

func TestRunFormat_SortedYAMLInput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.yaml")
	outputPath := filepath.Join(tempDir, "output.json")
	require.NoError(t, os.WriteFile(inputPath, []byte("openapi: 3.0.0\npaths:\n  /z: {}\n  /a: {}\n"), 0o600))

	err := runFormat(context.Background(), formatFlags{
		Schema: inputPath,
		Out:    outputPath,
		Style:  "sorted",
	})
	require.NoError(t, err)

	actual, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, `{
  "openapi": "3.0.0",
  "paths": {
    "/a": {},
    "/z": {}
  }
}
`, string(actual))
}

func TestRunFormat_SortedRejectsYAMLOutputBeforeCreate(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"openapi":"3.0.0"}`), 0o600))

	for _, outputName := range []string{"output.yaml", "output.YML"} {
		outputPath := filepath.Join(tempDir, outputName)
		err := runFormat(context.Background(), formatFlags{
			Schema: inputPath,
			Out:    outputPath,
			Style:  "sorted",
		})
		require.EqualError(t, err, "sorted formatting only supports JSON output")
		_, statErr := os.Stat(outputPath)
		require.ErrorIs(t, statErr, os.ErrNotExist, "rejected output should not be created")
	}
}

func TestRunFormat_RejectsUnknownStyleBeforeCreate(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "output.json")
	err := runFormat(context.Background(), formatFlags{
		Schema: "unused.json",
		Out:    outputPath,
		Style:  "unknown",
	})
	require.EqualError(t, err, `unsupported format style "unknown"`)
	_, statErr := os.Stat(outputPath)
	require.ErrorIs(t, statErr, os.ErrNotExist, "rejected output should not be created")
}
