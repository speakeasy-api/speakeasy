package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	cigit "github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectReleasePatchFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".speakeasy", "patches", "models"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".speakeasy", "patches", "sdk.go.patch"), []byte(`--- a/sdk.go
+++ b/sdk.go
@@ -1,2 +1,4 @@
 package testsdk
 
+// custom
+func sdkLabel() string { return "sdk" }
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".speakeasy", "patches", "models", "status.go.patch"), []byte(`--- a/models/status.go
+++ b/models/status.go
@@ -1,3 +1,4 @@
 package models
 
 type Status struct{}
+func (s *Status) String() string { return "status" }
`), 0o644))

	patchFiles, err := collectReleasePatchFiles(tempDir)
	require.NoError(t, err)
	assert.Equal(t, []patchFileSummary{
		{Path: "models/status.go.patch", Additions: 1, Deletions: 0},
		{Path: "sdk.go.patch", Additions: 2, Deletions: 0},
	}, patchFiles)
}

func TestBuildChangesetReleasePRBody_IncludesTables(t *testing.T) {
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("GITHUB_HEAD_REF", "")

	body := buildChangesetReleasePRBody(context.Background(), &cigit.Git{}, []changesetReleaseSummary{
		{
			TargetName: "go",
			Version:    "1.2.3",
			BumpType:   "minor",
			Operations: []string{"getStatus"},
			PatchFiles: []patchFileSummary{
				{Path: "models/status.go.patch", Additions: 1, Deletions: 0},
				{Path: "sdk.go.patch", Additions: 2, Deletions: 0},
			},
			Changesets: []*changeset.Changeset{
				{
					Version: changeset.VersionBumpMinor,
					Message: "Add status resources",
				},
			},
		},
	})

	assert.Contains(t, body, "## go@1.2.3")
	assert.Contains(t, body, "| File | Delta |")
	assert.Contains(t, body, "| `models/status.go.patch` | `+1 -0` |")
	assert.Contains(t, body, "| `sdk.go.patch` | `+2 -0` |")
	assert.Contains(t, body, "| Changeset | Branch | Bump | Summary |")
	assert.Contains(t, body, "Add status resources")
	assert.NotContains(t, body, "Released patch files:")
}
