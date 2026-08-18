package transform

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSortedDocument(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{"z":1,"openapi":"3.0.0"}`), 0o600))

	var output bytes.Buffer
	require.NoError(t, FormatSortedDocument(inputPath, false, &output))
	assert.Equal(t, "{\n  \"openapi\": \"3.0.0\",\n  \"z\": 1\n}\n", output.String())
}

func TestFormatSortedDocument_MissingInputReturnsError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := FormatSortedDocument(filepath.Join(t.TempDir(), "missing.json"), false, &output)
	require.Error(t, err)
	assert.Empty(t, output.String())
}

func TestFormatSortedFromReader_Success(t *testing.T) {
	t.Parallel()

	input := `{"paths":{"/z":{},"/a":{}},"openapi":"3.0.0","parameters":[{"name":"z"},{"name":"a"}]}`
	expected := `{
  "openapi": "3.0.0",
  "paths": {
    "/a": {},
    "/z": {}
  },
  "parameters": [
    {
      "name": "a"
    },
    {
      "name": "z"
    }
  ]
}
`

	var output bytes.Buffer
	err := FormatSortedFromReader(strings.NewReader(input), &output, false)
	require.NoError(t, err, "sorted formatting should succeed")
	assert.Equal(t, expected, output.String(), "sorted output should match")
}

func TestFormatSortedFromReader_YAMLInput(t *testing.T) {
	t.Parallel()

	input := `openapi: 3.0.0
paths:
  /z: {}
  /a: {}
components:
  schemas:
    Example:
      required: [z, a]
      properties:
        z:
          type: string
        a:
          type: string
responses:
  200:
    description: ok
`
	expected := `{
  "openapi": "3.0.0",
  "paths": {
    "/a": {},
    "/z": {}
  },
  "components": {
    "schemas": {
      "Example": {
        "properties": {
          "a": {
            "type": "string"
          },
          "z": {
            "type": "string"
          }
        },
        "required": [
          "a",
          "z"
        ]
      }
    }
  },
  "responses": {
    "200": {
      "description": "ok"
    }
  }
}
`

	var output bytes.Buffer
	err := FormatSortedFromReader(strings.NewReader(input), &output, false)
	require.NoError(t, err, "YAML input should be converted and sorted")
	assert.Equal(t, expected, output.String(), "YAML input should produce sorted JSON")
}

func TestFormatSortedFromReader_InvalidInputReturnsError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := FormatSortedFromReader(strings.NewReader("{not valid"), &output, false)
	require.Error(t, err, "invalid JSON and YAML should fail")
	assert.Empty(t, output.String(), "invalid input should not produce output")
}

func TestFormatSortedFromReader_ReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	var output bytes.Buffer
	err := FormatSortedFromReader(errorReader{err: readErr}, &output, false)
	require.ErrorIs(t, err, readErr)
	assert.Empty(t, output.String())
}

func TestFormatSortedFromReader_YAMLOutputReturnsError(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := FormatSortedFromReader(strings.NewReader(`{"openapi":"3.0.0"}`), &output, true)
	require.EqualError(t, err, "sorted formatting only supports JSON output", "YAML output should fail clearly")
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
