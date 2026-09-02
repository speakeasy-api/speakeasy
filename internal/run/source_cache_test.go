package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With --frozen-workflow-lockfile the source pipeline never writes
// Source.GetOutputLocation() (.speakeasy/temp/output_<hash>.yaml), so
// SourceResult.OutputPath points at a file that does not exist. The cached
// result served to the second and later targets sharing the source must hand
// back the document the pipeline actually produced, otherwise the generator
// stats the missing temp file, falls through to the remote-download branch and
// fails with `unsupported protocol scheme ""`.
func TestCachedSource_FrozenLockfileReturnsResolvedDocument(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	resolved := filepath.Join(tempDir, "merge_abc123.yaml")
	require.NoError(t, os.WriteFile(resolved, []byte("openapi: 3.0.0\ninfo:\n  title: t\n  version: 1.0.0\npaths: {}\n"), 0o644))
	neverWritten := filepath.Join(tempDir, "output_32fac1.yaml")

	w := &Workflow{
		FrozenWorkflowLock: true,
		SourceResults: map[string]*SourceResult{
			"my-source": {
				Source:       "my-source",
				OutputPath:   neverWritten,
				documentPath: resolved,
			},
		},
	}

	path, res, ok := w.cachedSource("my-source")
	require.True(t, ok)
	require.NotNil(t, res)
	assert.Equal(t, resolved, path)

	// The path handed to the generator must be an existing local file so that
	// it is never classified as a remote document.
	_, err := os.Stat(path)
	require.NoError(t, err)
	isRemote, contents, err := openapi.GetSchemaContents(t.Context(), path, "", "")
	require.NoError(t, err)
	assert.False(t, isRemote)
	assert.Contains(t, string(contents), "openapi: 3.0.0")
}

// A SourceResult that was recorded for a run that did not complete (for
// example a linting failure) carries an OutputPath but no resolved document.
// It must not be served from the cache: the next caller (another target, or
// the minimum-viable-spec retry) has to run the source again.
func TestCachedSource_IncompleteResultIsNotCached(t *testing.T) {
	t.Parallel()

	w := &Workflow{
		SourceResults: map[string]*SourceResult{
			"my-source": {
				Source:     "my-source",
				OutputPath: filepath.Join(t.TempDir(), "output_32fac1.yaml"),
			},
		},
	}

	path, res, ok := w.cachedSource("my-source")
	assert.False(t, ok)
	assert.Nil(t, res)
	assert.Empty(t, path)

	_, _, ok = w.cachedSource("unknown-source")
	assert.False(t, ok)
}
