package overlay

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	schemaFile               = "testdata/base.yaml"
	schemaJSON               = "testdata/base.json"
	overlayFile              = "testdata/overlay.yaml"
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

func test(t *testing.T, schemaFile string, expectedFile string, yamlOut bool) {
	ext := "json"
	if yamlOut {
		ext = "yaml"
	}
	tmpFile, err := os.CreateTemp("", "output."+ext)
	require.NoError(t, err)
	defer tmpFile.Close()

	err = Apply(schemaFile, overlayFile, yamlOut, tmpFile, true, false)
	assert.NoError(t, err)

	expectedContent, err := os.ReadFile(expectedFile)
	assert.NoError(t, err)

	actualContent, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	println(string(actualContent))

	assert.Equal(t, string(expectedContent), string(actualContent))
}
