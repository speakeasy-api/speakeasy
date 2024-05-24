
package overlay

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	schemaFile    = "testdata/base.yaml"
	overlayFile   = "testdata/overlay.yaml"
	expectedFile  = "testdata/expected.yaml"
)


func TestApply(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "output.yaml")
	require.NoError(t, err)
	defer tmpFile.Close()

	err = Apply(schemaFile, overlayFile, tmpFile)
	assert.NoError(t, err)

	expectedContent, err := os.ReadFile(expectedFile)
	assert.NoError(t, err)

	actualContent, err := os.ReadFile(tmpFile.Name())
	assert.NoError(t, err)

	assert.Equal(t, string(expectedContent), string(actualContent))
}