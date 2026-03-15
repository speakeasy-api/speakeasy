package patches

import (
	"os"
	"path/filepath"
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordMove(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	id := "aabbccddeeff"

	createTestFileWithID(t, tempDir, "src/new.go", id, "package foo")
	writeTestWorkspace(t, tempDir, lockfile.TrackedFile{ID: id}, "src/old.go")

	err := RecordMove(tempDir, "src/old.go", "src/new.go")
	require.NoError(t, err)

	cfg, err := config.Load(tempDir)
	require.NoError(t, err)

	tracked, ok := cfg.LockFile.TrackedFiles.Get("src/old.go")
	require.True(t, ok)
	assert.False(t, tracked.Deleted)
	assert.Equal(t, "src/new.go", GetMovedTo(tracked))
}

func TestRecordMoveRejectsMismatchedID(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	createTestFileWithID(t, tempDir, "src/new.go", "001122334455", "package foo")
	writeTestWorkspace(t, tempDir, lockfile.TrackedFile{ID: "aabbccddeeff"}, "src/old.go")

	err := RecordMove(tempDir, "src/old.go", "src/new.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected")
}

func writeTestWorkspace(t *testing.T, dir string, tracked lockfile.TrackedFile, trackedPath string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".speakeasy"), 0o755))

	cfg := &config.Configuration{
		ConfigVersion: config.Version,
	}
	require.NoError(t, config.SaveConfig(dir, cfg))

	lf := lockfile.New()
	lf.TrackedFiles.Set(trackedPath, tracked)
	require.NoError(t, config.SaveLockFile(dir, lf))
}
