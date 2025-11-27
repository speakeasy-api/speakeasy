package patches

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a test file with @generated-id header
// id should be a 12-char hex string (e.g., "a1b2c3d4e5f6")
func createTestFileWithID(t *testing.T, dir, relativePath, id, content string) string {
	fullPath := filepath.Join(dir, relativePath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)

	fileContent := fmt.Sprintf("// @generated-id: %s\n%s", id, content)
	err = os.WriteFile(fullPath, []byte(fileContent), 0644)
	require.NoError(t, err)

	return fullPath
}

// Helper to compute SHA-1 checksum of content in lockfile format (sha1:<hex>)
// Uses the same normalization as lockfile.ComputeFileChecksum
func computeChecksum(content string) string {
	sumHex, _ := lockfile.HashNormalizedSHA1(strings.NewReader(content))
	return "sha1:" + sumHex
}

func TestDetectFileChanges_NilLockFile(t *testing.T) {
	isDirty, modifiedPaths, err := DetectFileChanges("/tmp/test", nil)
	assert.NoError(t, err)
	assert.False(t, isDirty)
	assert.Empty(t, modifiedPaths)
}

func TestDetectFileChanges_NilTrackedFiles(t *testing.T) {
	lockFile := &config.LockFile{
		LockVersion:  lockfile.LockV2,
		TrackedFiles: nil,
	}
	isDirty, modifiedPaths, err := DetectFileChanges("/tmp/test", lockFile)
	assert.NoError(t, err)
	assert.False(t, isDirty)
	assert.Empty(t, modifiedPaths)
}

func TestDetectFileChanges_DeletedFile(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up lockfile with a tracked file that doesn't exist on disk
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/deleted.go", lockfile.TrackedFile{
		ID: "aabbccddeeff", // 12 hex chars
	})

	// Detect changes - file doesn't exist, should be marked dirty
	isDirty, modifiedPaths, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file is deleted")
	assert.Empty(t, modifiedPaths, "No modified paths for deleted file")

	// Verify the tracked file is marked as deleted
	tracked, ok := lf.TrackedFiles.Get("src/deleted.go")
	require.True(t, ok)
	assert.True(t, tracked.Deleted, "TrackedFile.Deleted should be true")
	assert.Empty(t, tracked.MovedTo, "TrackedFile.MovedTo should be empty")
}

func TestDetectFileChanges_MovedFile(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	id := "112233445566" // 12 hex chars

	// Create file at NEW location with the ID
	createTestFileWithID(t, tempDir, "src/newlocation.go", id, "package foo")

	// Set up lockfile with file tracked at OLD location
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/oldlocation.go", lockfile.TrackedFile{
		ID: id,
	})

	// Detect changes - ID found at different path
	isDirty, modifiedPaths, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file is moved")
	assert.Empty(t, modifiedPaths, "No modified paths for moved file")

	// Verify the tracked file has MovedTo set
	tracked, ok := lf.TrackedFiles.Get("src/oldlocation.go")
	require.True(t, ok)
	assert.False(t, tracked.Deleted, "TrackedFile.Deleted should be false")
	assert.Equal(t, "src/newlocation.go", tracked.MovedTo, "TrackedFile.MovedTo should point to new location")
}

func TestDetectFileChanges_ModifiedFile(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	id := "223344556677" // 12 hex chars
	originalContent := "// @generated-id: " + id + "\npackage foo\n\nfunc Original() {}\n"
	modifiedContent := "// @generated-id: " + id + "\npackage foo\n\nfunc Modified() {}\n"

	// Create file on disk with MODIFIED content
	fullPath := filepath.Join(tempDir, "src/modified.go")
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Set up lockfile with checksum of ORIGINAL content
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/modified.go", lockfile.TrackedFile{
		ID:                id,
		LastWriteChecksum: computeChecksum(originalContent),
	})

	// Detect changes - checksum differs
	isDirty, modifiedPaths, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file checksum differs")
	assert.Contains(t, modifiedPaths, "src/modified.go", "Should include modified file in modifiedPaths")

	// File should not be marked as deleted or moved
	tracked, ok := lf.TrackedFiles.Get("src/modified.go")
	require.True(t, ok)
	assert.False(t, tracked.Deleted)
	assert.Empty(t, tracked.MovedTo)
}

func TestDetectFileChanges_UnchangedFiles(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	id := "334455667788" // 12 hex chars
	content := "// @generated-id: " + id + "\npackage foo\n"

	// Create file on disk
	fullPath := filepath.Join(tempDir, "src/unchanged.go")
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set up lockfile with matching checksum
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/unchanged.go", lockfile.TrackedFile{
		ID:                id,
		LastWriteChecksum: computeChecksum(content),
	})

	// Detect changes - nothing changed
	isDirty, modifiedPaths, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.False(t, isDirty, "Should NOT be dirty when file is unchanged")
	assert.Empty(t, modifiedPaths, "No modified paths for unchanged file")
}

func TestDetectFileChanges_ClearsStaleMarkers(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	id := "445566778899" // 12 hex chars

	// Create file at expected location
	createTestFileWithID(t, tempDir, "src/restored.go", id, "package foo")

	// Set up lockfile with stale Deleted marker
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/restored.go", lockfile.TrackedFile{
		ID:      id,
		Deleted: true, // Stale marker from previous state
	})

	// Detect changes - file exists at expected location now
	isDirty, modifiedPaths, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	// Not dirty because no actual change detected (no checksum to compare)
	// But stale markers should be cleared
	assert.False(t, isDirty)
	assert.Empty(t, modifiedPaths)

	// Verify stale markers are cleared
	tracked, ok := lf.TrackedFiles.Get("src/restored.go")
	require.True(t, ok)
	assert.False(t, tracked.Deleted, "Stale Deleted marker should be cleared")
	assert.Empty(t, tracked.MovedTo, "MovedTo should be empty")
}

func TestDetectFileChanges_MultipleFiles(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	id1 := "556677889900" // 12 hex chars
	id2 := "667788990011" // 12 hex chars
	id3 := "778899001122" // 12 hex chars

	// Create file1 at expected location (unchanged)
	createTestFileWithID(t, tempDir, "src/file1.go", id1, "package foo")

	// Create file2 at NEW location (moved)
	createTestFileWithID(t, tempDir, "src/newdir/file2.go", id2, "package bar")

	// file3 doesn't exist on disk (deleted)

	// Set up lockfile
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/file1.go", lockfile.TrackedFile{ID: id1})
	lf.TrackedFiles.Set("src/file2.go", lockfile.TrackedFile{ID: id2})
	lf.TrackedFiles.Set("src/file3.go", lockfile.TrackedFile{ID: id3})

	// Detect changes
	isDirty, _, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when some files are moved/deleted")

	// Check file1 - unchanged
	tracked1, _ := lf.TrackedFiles.Get("src/file1.go")
	assert.False(t, tracked1.Deleted)
	assert.Empty(t, tracked1.MovedTo)

	// Check file2 - moved
	tracked2, _ := lf.TrackedFiles.Get("src/file2.go")
	assert.False(t, tracked2.Deleted)
	assert.Equal(t, "src/newdir/file2.go", tracked2.MovedTo)

	// Check file3 - deleted
	tracked3, _ := lf.TrackedFiles.Get("src/file3.go")
	assert.True(t, tracked3.Deleted)
	assert.Empty(t, tracked3.MovedTo)
}

func TestGetFileChangeSummary_Empty(t *testing.T) {
	summary := GetFileChangeSummary(nil)
	assert.Empty(t, summary.Deleted)
	assert.Empty(t, summary.Moved)
	assert.Empty(t, summary.Modified)
	assert.True(t, summary.IsEmpty())
}

func TestGetFileChangeSummary_WithChanges(t *testing.T) {
	lf := lockfile.New()
	lf.TrackedFiles.Set("deleted1.go", lockfile.TrackedFile{Deleted: true})
	lf.TrackedFiles.Set("deleted2.go", lockfile.TrackedFile{Deleted: true})
	lf.TrackedFiles.Set("moved.go", lockfile.TrackedFile{MovedTo: "newpath/moved.go"})
	lf.TrackedFiles.Set("unchanged.go", lockfile.TrackedFile{})

	summary := GetFileChangeSummary(lf)

	assert.Len(t, summary.Deleted, 2)
	assert.Contains(t, summary.Deleted, "deleted1.go")
	assert.Contains(t, summary.Deleted, "deleted2.go")

	assert.Len(t, summary.Moved, 1)
	assert.Equal(t, "newpath/moved.go", summary.Moved["moved.go"])

	assert.False(t, summary.IsEmpty())
}

func TestFormatSummary_Empty(t *testing.T) {
	summary := FileChangeSummary{
		Moved: make(map[string]string),
	}
	result := summary.FormatSummary(10, false)
	assert.Empty(t, result)
}

func TestFormatSummary_WithChanges(t *testing.T) {
	summary := FileChangeSummary{
		Deleted:  []string{"file1.go", "file2.go"},
		Moved:    map[string]string{"old.go": "new.go"},
		Modified: []FileDiff{{Path: "changed.go"}},
	}

	result := summary.FormatSummary(10, false)

	assert.Contains(t, result, "D file1.go")
	assert.Contains(t, result, "D file2.go")
	assert.Contains(t, result, "R old.go -> new.go")
	assert.Contains(t, result, "M changed.go")
}

func TestFormatSummary_Truncation(t *testing.T) {
	summary := FileChangeSummary{
		Deleted: []string{"f1.go", "f2.go", "f3.go", "f4.go", "f5.go"},
		Moved:   make(map[string]string),
	}

	result := summary.FormatSummary(3, false)

	// Should show 3 lines plus truncation message
	lines := 0
	for _, c := range result {
		if c == '\n' {
			lines++
		}
	}
	// 3 items + "... and X more" = 4 lines, so 3 newlines
	assert.Equal(t, 3, lines)
	assert.Contains(t, result, "... and 2 more")
}

