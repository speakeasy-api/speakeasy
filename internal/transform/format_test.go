package transform

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFormat(t *testing.T) {
	// Create a buffer to store the formatted spec
	var testInput bytes.Buffer
	var testOutput bytes.Buffer

	// Call FormatDocument to format the spec
	err := FormatDocument(context.Background(), "../../integration/resources/unformatted.yaml", true, &testInput)
	require.NoError(t, err)

	// Parse the formatted spec
	formattedDoc, err := libopenapi.NewDocument(testInput.Bytes())
	require.NoError(t, err)

	// Check that the formatted spec is valid
	_, errors := formattedDoc.BuildV3Model()
	require.Empty(t, errors)

	// Open the spec we expect to see to compare
	file, err := os.Open("../../integration/resources/formatted.yaml")
	require.NoError(t, err)
	defer file.Close()

	// Read the expected spec into a buffer
	reader := bufio.NewReader(file)
	testOutput.ReadFrom(reader)
	require.NoError(t, err)

	var actual yaml.Node
	var expected yaml.Node

	err = yaml.Unmarshal(testInput.Bytes(), &actual)
	require.NoError(t, err)

	err = yaml.Unmarshal(testOutput.Bytes(), &expected)
	require.NoError(t, err)

	// Require the pre-formatted spec matches the expected spec
	require.Equal(t, expected, actual)
}
