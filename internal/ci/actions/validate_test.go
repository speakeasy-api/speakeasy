package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverSpecPaths_RecursiveGlob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	topLevel := filepath.Join(root, "specs", "payments", "openapi.yaml")
	nested := filepath.Join(root, "specs", "team", "internal", "openapi.yaml")

	require.NoError(t, os.MkdirAll(filepath.Dir(topLevel), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(nested), 0o755))
	require.NoError(t, os.WriteFile(topLevel, []byte("openapi: 3.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(nested, []byte("openapi: 3.1.0\n"), 0o644))

	matches, err := discoverSpecPaths([]string{filepath.Join(root, "specs", "**", "*.yaml")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{topLevel, nested}, matches)
}

func TestDiscoverSpecPaths_NoMatches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	matches, err := discoverSpecPaths([]string{filepath.Join(root, "specs", "**", "*.yaml")})
	require.Error(t, err)
	assert.Nil(t, matches)
	assert.Contains(t, err.Error(), "no spec files found matching pattern")
}
