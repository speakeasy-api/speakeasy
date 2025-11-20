package merging

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/epiclabs-io/diff3"
)

// TextMerger implements a text-based 3-way merge using the diff3 algorithm.
// It performs proper 3-way merge with automatic resolution of non-overlapping changes
// and injection of Git-style conflict markers for overlapping changes.
type TextMerger struct{}

func NewTextMerger() *TextMerger {
	return &TextMerger{}
}

// Merge performs a 3-way merge: Merge(base, current, new)
// - base: Pure generated content from previous run (merge base)
// - current: File on disk with user edits
// - new: Fresh generated content
//
// Returns:
// - MergeStatusClean: All changes merged successfully
// - MergeStatusConflict: Overlapping changes, conflict markers injected
// - MergeStatusCreated: New file (no base)
// - MergeStatusFastForward: Current equals new (no changes needed)
func (m *TextMerger) Merge(base, current, new []byte) (*MergeResult, error) {
	res := &MergeResult{
		Status: MergeStatusClean,
	}

	// 1. Fast path: Identical content
	if bytes.Equal(current, new) {
		res.Content = current
		res.Status = MergeStatusFastForward
		return res, nil
	}

	// 2. No base: Treat as new file
	if base == nil || len(base) == 0 {
		// No history - this is a new file or we lost tracking
		// Generator is authoritative for new files
		res.Content = new
		res.Status = MergeStatusCreated
		return res, nil
	}

	// 3. Check if current equals base (user made no changes)
	if bytes.Equal(current, base) {
		// Fast-forward: Just use new content
		res.Content = new
		res.Status = MergeStatusFastForward
		return res, nil
	}

	// 4. Check if new equals base (generator made no changes)
	if bytes.Equal(new, base) {
		// No generator changes, keep user's version
		res.Content = current
		res.Status = MergeStatusClean
		return res, nil
	}

	// 5. Perform 3-way merge using diff3
	result, err := diff3.Merge(
		strings.NewReader(string(current)), // A (ours/current/user)
		strings.NewReader(string(base)),    // O (original/base)
		strings.NewReader(string(new)),     // B (theirs/new/generator)
		true,                                // includeConflicts - inject markers
		"CURRENT (User's changes)",
		"NEW (Generated code)",
	)
	if err != nil {
		return nil, fmt.Errorf("diff3 merge failed: %w", err)
	}

	// 6. Read merged content
	mergedBytes, err := io.ReadAll(result.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to read merge result: %w", err)
	}

	res.Content = mergedBytes

	// 7. Check for conflicts
	if result.Conflicts {
		res.Status = MergeStatusConflict
		res.HasConflicts = true

		// Parse conflict markers to populate Conflicts slice
		// This helps the engine report conflict locations
		res.Conflicts = parseConflictMarkers(string(mergedBytes))
	} else {
		res.Status = MergeStatusClean
	}

	return res, nil
}

// parseConflictMarkers scans the merged content for conflict markers
// and returns structured conflict information
func parseConflictMarkers(content string) []Conflict {
	var conflicts []Conflict
	lines := strings.Split(content, "\n")

	var inConflict bool
	var startLine int

	for i, line := range lines {
		if strings.HasPrefix(line, "<<<<<<<") {
			inConflict = true
			startLine = i + 1 // Line numbers are 1-indexed
		} else if strings.HasPrefix(line, ">>>>>>>") && inConflict {
			conflicts = append(conflicts, Conflict{
				StartLine: startLine,
				EndLine:   i + 1,
				Message:   "Overlapping changes between user edits and generated code",
			})
			inConflict = false
		}
	}

	return conflicts
}
