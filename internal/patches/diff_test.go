package patches

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "text content",
			content:  []byte("hello world\nthis is text"),
			expected: false,
		},
		{
			name:     "binary with null byte",
			content:  []byte("hello\x00world"),
			expected: true,
		},
		{
			name:     "empty content",
			content:  []byte{},
			expected: false,
		},
		{
			name:     "binary at start",
			content:  []byte{0x00, 0x01, 0x02},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := isBinary(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "LF unchanged",
			input:    "hello\nworld",
			expected: "hello\nworld",
		},
		{
			name:     "CRLF to LF",
			input:    "hello\r\nworld",
			expected: "hello\nworld",
		},
		{
			name:     "CR to LF",
			input:    "hello\rworld",
			expected: "hello\nworld",
		},
		{
			name:     "mixed endings",
			input:    "line1\r\nline2\rline3\n",
			expected: "line1\nline2\nline3\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := normalizeLineEndings(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCountDiffStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		diffText string
		expected DiffStats
	}{
		{
			name:     "empty diff",
			diffText: "",
			expected: DiffStats{Added: 0, Removed: 0},
		},
		{
			name: "simple diff",
			diffText: `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 unchanged
-removed line
+added line
 unchanged`,
			expected: DiffStats{Added: 1, Removed: 1},
		},
		{
			name: "multiple additions",
			diffText: `--- a/file.go
+++ b/file.go
@@ -1 +1,4 @@
 unchanged
+added1
+added2
+added3`,
			expected: DiffStats{Added: 3, Removed: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := countDiffStats(tc.diffText)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestComputeFileDiff_NoPristine(t *testing.T) {
	t.Parallel()

	fd := ComputeFileDiff("/tmp", "test.go", "", nil)
	assert.Equal(t, "test.go", fd.Path)
	assert.Equal(t, "(no pristine base available)", fd.DiffText)
}

func TestComputeFileDiff_NoGitRepo(t *testing.T) {
	t.Parallel()

	fd := ComputeFileDiff("/tmp", "test.go", "abc123", nil)
	assert.Equal(t, "(git repository not available)", fd.DiffText)
}

// mockGitRepo implements GitRepository for testing
type mockGitRepo struct {
	blobs map[string][]byte
	isNil bool
}

func (m *mockGitRepo) IsNil() bool                         { return m.isNil }
func (m *mockGitRepo) Root() string                        { return "" }
func (m *mockGitRepo) HasObject(hash string) bool          { _, ok := m.blobs[hash]; return ok }
func (m *mockGitRepo) GetBlob(hash string) ([]byte, error) { return m.blobs[hash], nil }
func (m *mockGitRepo) WriteBlob(content []byte) (string, error) {
	return "", nil
}
func (m *mockGitRepo) WriteTree(entries []TreeEntry) (string, error) { return "", nil }
func (m *mockGitRepo) CommitTree(treeHash, parentHash, message string) (string, error) {
	return "", nil
}
func (m *mockGitRepo) GetRef(refName string) (string, error)            { return "", nil }
func (m *mockGitRepo) UpdateRef(refName, newHash, oldHash string) error { return nil }
func (m *mockGitRepo) FetchRef(refSpec string) error                    { return nil }
func (m *mockGitRepo) PushRef(refSpec string) error                     { return nil }
func (m *mockGitRepo) SetConflictState(path string, base, ours, theirs []byte, isExecutable bool) error {
	return nil
}

func TestComputeFileDiff_WithDiff(t *testing.T) {
	t.Parallel()

	// Create temp directory with test file
	tempDir := t.TempDir()

	// Write current file
	currentContent := "package foo\n\nfunc Modified() {}\n"
	err := os.WriteFile(filepath.Join(tempDir, "test.go"), []byte(currentContent), 0o644)
	require.NoError(t, err)

	// Create mock git repo with pristine content
	pristineContent := "package foo\n\nfunc Original() {}\n"
	repo := &mockGitRepo{
		blobs: map[string][]byte{
			"abc123": []byte(pristineContent),
		},
	}

	fd := ComputeFileDiff(tempDir, "test.go", "abc123", repo)
	require.NoError(t, err)

	assert.Equal(t, "test.go", fd.Path)
	assert.Equal(t, "abc123", fd.PristineHash)
	assert.Contains(t, fd.DiffText, "-func Original()")
	assert.Contains(t, fd.DiffText, "+func Modified()")
	assert.Equal(t, 1, fd.Stats.Added)
	assert.Equal(t, 1, fd.Stats.Removed)
}

func TestFormatSummary_WithDiffs(t *testing.T) {
	t.Parallel()

	summary := FileChangeSummary{
		Modified: []FileDiff{
			{
				Path: "changed.go",
				Stats: DiffStats{
					Added:   5,
					Removed: 2,
				},
				DiffText: "@@ -1,3 +1,6 @@\n unchanged\n-removed\n+added1\n+added2",
			},
		},
		Moved: make(map[string]string),
	}

	// Without diffs
	result := summary.FormatSummary(10, false)
	assert.Contains(t, result, "M changed.go")
	assert.NotContains(t, result, "+5/-2")

	// With diffs
	result = summary.FormatSummary(20, true)
	assert.Contains(t, result, "M changed.go (+5/-2)")
	assert.Contains(t, result, "@@")
}
