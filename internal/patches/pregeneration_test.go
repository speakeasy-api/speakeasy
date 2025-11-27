package patches

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a test file with @generated-id header
func createTestFileWithUUID(t *testing.T, dir, relativePath, uuid, content string) string {
	fullPath := filepath.Join(dir, relativePath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)

	fileContent := fmt.Sprintf("// @generated-id: %s\n%s", uuid, content)
	err = os.WriteFile(fullPath, []byte(fileContent), 0644)
	require.NoError(t, err)

	return fullPath
}

// Helper to compute SHA-1 checksum of content
func computeChecksum(content string) string {
	hash := sha1.New()
	hash.Write([]byte(content))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func TestDetectFileChanges_NilLockFile(t *testing.T) {
	isDirty, err := DetectFileChanges("/tmp/test", nil)
	assert.NoError(t, err)
	assert.False(t, isDirty)
}

func TestDetectFileChanges_NilTrackedFiles(t *testing.T) {
	lockFile := &config.LockFile{
		LockVersion:  lockfile.LockV2,
		TrackedFiles: nil,
	}
	isDirty, err := DetectFileChanges("/tmp/test", lockFile)
	assert.NoError(t, err)
	assert.False(t, isDirty)
}

func TestDetectFileChanges_DeletedFile(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up lockfile with a tracked file that doesn't exist on disk
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/deleted.go", lockfile.TrackedFile{
		ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})

	// Detect changes - file doesn't exist, should be marked dirty
	isDirty, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file is deleted")

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

	uuid := "11111111-2222-3333-4444-555555555555"

	// Create file at NEW location with the UUID
	createTestFileWithUUID(t, tempDir, "src/newlocation.go", uuid, "package foo")

	// Set up lockfile with file tracked at OLD location
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/oldlocation.go", lockfile.TrackedFile{
		ID: uuid,
	})

	// Detect changes - UUID found at different path
	isDirty, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file is moved")

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

	uuid := "22222222-3333-4444-5555-666666666666"
	originalContent := "// @generated-id: " + uuid + "\npackage foo\n\nfunc Original() {}\n"
	modifiedContent := "// @generated-id: " + uuid + "\npackage foo\n\nfunc Modified() {}\n"

	// Create file on disk with MODIFIED content
	fullPath := filepath.Join(tempDir, "src/modified.go")
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Set up lockfile with checksum of ORIGINAL content
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/modified.go", lockfile.TrackedFile{
		ID:                uuid,
		LastWriteChecksum: computeChecksum(originalContent),
	})

	// Detect changes - checksum differs
	isDirty, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.True(t, isDirty, "Should be dirty when file checksum differs")

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

	uuid := "33333333-4444-5555-6666-777777777777"
	content := "// @generated-id: " + uuid + "\npackage foo\n"

	// Create file on disk
	fullPath := filepath.Join(tempDir, "src/unchanged.go")
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set up lockfile with matching checksum
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/unchanged.go", lockfile.TrackedFile{
		ID:                uuid,
		LastWriteChecksum: computeChecksum(content),
	})

	// Detect changes - nothing changed
	isDirty, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	assert.False(t, isDirty, "Should NOT be dirty when file is unchanged")
}

func TestDetectFileChanges_ClearsStaleMarkers(t *testing.T) {
	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	uuid := "44444444-5555-6666-7777-888888888888"

	// Create file at expected location
	createTestFileWithUUID(t, tempDir, "src/restored.go", uuid, "package foo")

	// Set up lockfile with stale Deleted marker
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/restored.go", lockfile.TrackedFile{
		ID:      uuid,
		Deleted: true, // Stale marker from previous state
	})

	// Detect changes - file exists at expected location now
	isDirty, err := DetectFileChanges(tempDir, lf)
	require.NoError(t, err)
	// Not dirty because no actual change detected (no checksum to compare)
	// But stale markers should be cleared
	assert.False(t, isDirty)

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

	uuid1 := "55555555-6666-7777-8888-999999999999"
	uuid2 := "66666666-7777-8888-9999-aaaaaaaaaaaa"
	uuid3 := "77777777-8888-9999-aaaa-bbbbbbbbbbbb"

	// Create file1 at expected location (unchanged)
	createTestFileWithUUID(t, tempDir, "src/file1.go", uuid1, "package foo")

	// Create file2 at NEW location (moved)
	createTestFileWithUUID(t, tempDir, "src/newdir/file2.go", uuid2, "package bar")

	// file3 doesn't exist on disk (deleted)

	// Set up lockfile
	lf := lockfile.New()
	lf.TrackedFiles.Set("src/file1.go", lockfile.TrackedFile{ID: uuid1})
	lf.TrackedFiles.Set("src/file2.go", lockfile.TrackedFile{ID: uuid2})
	lf.TrackedFiles.Set("src/file3.go", lockfile.TrackedFile{ID: uuid3})

	// Detect changes
	isDirty, err := DetectFileChanges(tempDir, lf)
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
	result := summary.FormatSummary(10)
	assert.Empty(t, result)
}

func TestFormatSummary_WithChanges(t *testing.T) {
	summary := FileChangeSummary{
		Deleted:  []string{"file1.go", "file2.go"},
		Moved:    map[string]string{"old.go": "new.go"},
		Modified: []string{"changed.go"},
	}

	result := summary.FormatSummary(10)

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

	result := summary.FormatSummary(3)

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

func TestComputeFileChecksum(t *testing.T) {
	// Create a temp file
	tempDir, err := os.MkdirTemp("", "patches-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	content := "Hello, World!"
	filePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Compute checksum
	checksum, err := computeFileChecksum(filePath)
	require.NoError(t, err)

	// Verify against known SHA-1 of "Hello, World!"
	expectedChecksum := "0a0a9f2a6772942557ab5355d76af442f8f65e01"
	assert.Equal(t, expectedChecksum, checksum)
}

func TestComputeFileChecksum_FileNotFound(t *testing.T) {
	_, err := computeFileChecksum("/nonexistent/path/file.txt")
	assert.Error(t, err)
}
