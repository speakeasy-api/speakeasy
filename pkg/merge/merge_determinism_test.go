package merge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/stretchr/testify/require"
)

func Test_merge_determinism(t *testing.T) {
	t.Parallel()

	// test data not included
	t.Skip()
	absSchemas := [][]byte{}
	wd, err := os.Getwd()
	require.NoError(t, err)
	files, err := os.ReadDir(filepath.Join(wd, "testdata"))
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}
		content, err := os.ReadFile(filepath.Join("testdata", f.Name()))
		require.NoError(t, err)
		absSchemas = append(absSchemas, content)
	}

	// Run merge twice and ensure the output is the same.
	got1, err := merge(t.Context(), absSchemas, nil, true)
	require.NoError(t, err)
	got2, err := merge(t.Context(), absSchemas, nil, true)
	require.NoError(t, err)

	// Verify both outputs parse as valid OpenAPI documents
	_, _, err = openapi.Unmarshal(context.Background(), bytes.NewReader(got1), openapi.WithSkipValidation())
	require.NoError(t, err)
	_, _, err = openapi.Unmarshal(context.Background(), bytes.NewReader(got2), openapi.WithSkipValidation())
	require.NoError(t, err)

	// Compare outputs for determinism
	require.Equal(t, string(got1), string(got2))
}
