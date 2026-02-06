package transform

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	// Create a buffer to store the normalized spec
	var testInput bytes.Buffer
	var testOutput bytes.Buffer

	// Call NormalizeDocument to normalize the spec
	err := NormalizeDocument(context.Background(), "../../integration/resources/normalize-input.yaml", true, true, &testInput)
	require.NoError(t, err)

	// Parse the normalized spec to verify it's valid
	_, _, err = openapi.Unmarshal(context.Background(), &testInput, openapi.WithSkipValidation())
	require.NoError(t, err)

	// Reset buffer position for comparison
	testInput.Reset()
	err = NormalizeDocument(context.Background(), "../../integration/resources/normalize-input.yaml", true, true, &testInput)
	require.NoError(t, err)

	// Open the spec we expect to see to compare
	file, err := os.Open("../../integration/resources/normalize-output.yaml")
	require.NoError(t, err)
	defer file.Close()

	// Read the expected spec into a buffer
	reader := bufio.NewReader(file)
	_, _ = testOutput.ReadFrom(reader)
	require.NoError(t, err)

	// Compare the content (trimmed to handle whitespace differences)
	require.Equal(t, testOutput.String(), testInput.String())
}

func TestNormalizeNoPrefixItems(t *testing.T) {
	t.Parallel()

	// Create a buffer to store the normalized spec
	var testInput bytes.Buffer
	var testOutput bytes.Buffer

	// Call NormalizeDocument to normalize the spec (without prefix items normalization)
	err := NormalizeDocument(context.Background(), "../../integration/resources/normalize-input.yaml", false, true, &testInput)
	require.NoError(t, err)

	// Parse the normalized spec to verify it's valid
	_, _, err = openapi.Unmarshal(context.Background(), &testInput, openapi.WithSkipValidation())
	require.NoError(t, err)

	// Reset buffer position for comparison
	testInput.Reset()
	err = NormalizeDocument(context.Background(), "../../integration/resources/normalize-input.yaml", false, true, &testInput)
	require.NoError(t, err)

	// Open the spec we expect to see to compare (should be same as input when no normalization)
	file, err := os.Open("../../integration/resources/normalize-input.yaml")
	require.NoError(t, err)
	defer file.Close()

	// Read the expected spec into a buffer
	reader := bufio.NewReader(file)
	_, _ = testOutput.ReadFrom(reader)
	require.NoError(t, err)

	// Compare the content
	require.Equal(t, testOutput.String(), testInput.String())
}
