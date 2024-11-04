package overlay

import (
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	schemaFile               = "testdata/base.yaml"
	schemaJSON               = "testdata/base.json"
	overlayFile              = "testdata/overlay.yaml"
	overlayStrictFailure     = "testdata/strict-failure.yaml"
	expectedFile             = "testdata/expected.yaml"
	expectedFileJSON         = "testdata/expected.json"
	expectedFileYAMLFromJSON = "testdata/expectedWrapped.yaml"
)

func TestApply_inYAML_outYAML(t *testing.T) {
	test(t, schemaFile, expectedFile, true)
}

func TestApply_inJSON_outJSON(t *testing.T) {
	test(t, schemaJSON, expectedFileJSON, false)
}

func TestApply_inYAML_outJSON(t *testing.T) {
	test(t, schemaFile, expectedFileJSON, false)
}

func TestApply_inJSON_outYAML(t *testing.T) {
	test(t, schemaJSON, expectedFileYAMLFromJSON, true)
}

func TestApply_StrictFailure(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "output.yaml")
	require.NoError(t, err)
	_, err = Apply(schemaFile, overlayStrictFailure, true, tmpFile, true, true)
	assert.Errorf(t, err, "unknown-element")
}

func test(t *testing.T, schemaFile string, expectedFile string, yamlOut bool) {
	ext := "json"
	if yamlOut {
		ext = "yaml"
	}
	tmpFile, err := os.CreateTemp("", "output."+ext)
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = Apply(schemaFile, overlayFile, yamlOut, tmpFile, true, false)
	assert.NoError(t, err)

	expectedContent, err := os.ReadFile(expectedFile)
	assert.NoError(t, err)

	actualContent, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	println(string(actualContent))

	normalizeLineEndings := func(s string) string {
		// Important on Windows
		return strings.ReplaceAll(s, "\r\n", "\n")
	}

	assert.Equal(t, normalizeLineEndings(string(expectedContent)), normalizeLineEndings(string(actualContent)))
}
