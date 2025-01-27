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
	schemaJSON    = "testdata/base.json"
	overlayFileV1 = "testdata/overlay.yaml"
	overlayFileV2 = "testdata/overlay-v2.yaml"
	overlayStrictFailure     = "testdata/strict-failure.yaml"
	expectedFile             = "testdata/expected.yaml"
	expectedFileJSON         = "testdata/expected.json"
	expectedFileYAMLFromJSON = "testdata/expectedWrapped.yaml"
)

func TestApply_inYAML_outYAML(t *testing.T) {
	test(t, schemaFile, overlayFileV1, expectedFile, true)
}

func TestApply_inJSON_outJSON(t *testing.T) {
	test(t, schemaJSON, overlayFileV1, expectedFileJSON, false)
}

func TestApply_inYAML_outJSON(t *testing.T) {
	test(t, schemaFile, overlayFileV1, expectedFileJSON, false)
}

func TestApply_inJSON_outYAML(t *testing.T) {
	test(t, schemaJSON, overlayFileV1, expectedFileYAMLFromJSON, true)
}

func TestApply_inYAML_outYAML_v2(t *testing.T) {
	test(t, schemaFile, overlayFileV2, expectedFile, true)
}

func TestApply_inJSON_outJSON_v2(t *testing.T) {
	test(t, schemaJSON, overlayFileV2, expectedFileJSON, false)
}

func TestApply_inYAML_outJSON_v2(t *testing.T) {
	test(t, schemaFile, overlayFileV2, expectedFileJSON, false)
}

func TestApply_inJSON_outYAML_v2(t *testing.T) {
	test(t, schemaJSON, overlayFileV2, expectedFileYAMLFromJSON, true)
}

func TestApply_StrictFailure(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "output.yaml")
	require.NoError(t, err)
	_, err = Apply(schemaFile, overlayStrictFailure, true, tmpFile, true, true)
	assert.Errorf(t, err, "unknown-element")
}

func test(t *testing.T, schemaFile string, overlayFile string, expectedFile string, yamlOut bool) {
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
