package patches

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// FileDiff represents a modified file with its diff
type FileDiff struct {
	Path         string
	PristineHash string
	DiffText     string
	Stats        DiffStats
}

// DiffStats contains statistics about a diff
type DiffStats struct {
	Added   int
	Removed int
}

// ComputeFileDiff generates a unified diff between pristine and current content
func ComputeFileDiff(outDir, path, pristineHash string, gitRepo GitRepository) (FileDiff, error) {
	fd := FileDiff{Path: path, PristineHash: pristineHash}

	// Handle missing pristine (first generation or legacy lockfile)
	if pristineHash == "" {
		fd.DiffText = "(no pristine base available)"
		return fd, nil
	}

	// Handle missing git repo
	if gitRepo == nil || gitRepo.IsNil() {
		fd.DiffText = "(git repository not available)"
		return fd, nil
	}

	// Get pristine content from git
	pristine, err := gitRepo.GetBlob(pristineHash)
	if err != nil {
		fd.DiffText = "(pristine object not found in git)"
		return fd, nil
	}

	// Get current content from disk
	current, err := os.ReadFile(filepath.Join(outDir, path))
	if err != nil {
		fd.DiffText = "(file not found on disk)"
		return fd, nil
	}

	// Skip binary files
	if isBinary(pristine) || isBinary(current) {
		fd.DiffText = "(binary file)"
		return fd, nil
	}

	// Normalize line endings
	pristineStr := normalizeLineEndings(string(pristine))
	currentStr := normalizeLineEndings(string(current))

	// Compute unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(pristineStr),
		B:        difflib.SplitLines(currentStr),
		FromFile: "generated",
		ToFile:   "current",
		Context:  3,
	}

	diffText, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		fd.DiffText = "(diff computation failed)"
		return fd, nil
	}

	fd.DiffText = diffText
	fd.Stats = countDiffStats(diffText)
	return fd, nil
}

// isBinary returns true if the content appears to be binary (contains null bytes).
func isBinary(content []byte) bool {
	// Check first 512 bytes for null byte
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// normalizeLineEndings converts all line endings to LF.
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// countDiffStats counts added and removed lines in a unified diff.
func countDiffStats(diffText string) DiffStats {
	var stats DiffStats
	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Removed++
		}
	}
	return stats
}
