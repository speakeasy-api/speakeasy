package merging

import (
	"bytes"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// TextMerger implements a text-based 3-way merge using diff-match-patch.
// It treats "Current" (User) as the destination and applies "New" (Generator) changes as patches.
type TextMerger struct {
	engine *dmp.DiffMatchPatch
}

func NewTextMerger() *TextMerger {
	return &TextMerger{
		engine: dmp.New(),
	}
}

func (m *TextMerger) Merge(base, current, new []byte) (*MergeResult, error) {
	res := &MergeResult{
		Status: MergeStatusClean,
	}

	// 1. Fast paths
	if bytes.Equal(current, new) {
		// Identical
		res.Content = current
		return res, nil
	}
	if base == nil || len(base) == 0 {
		// No base means this is effectively a new file or we lost history.
		// If User has content, and Generator has content, and they differ -> Conflict/Overwrite?
		// For safety in RTE, if we have no base, we assume Generator is authoritative
		// BUT if user file exists, we might want to warn.
		// Standard behavior: Overwrite if no history tracking.
		res.Content = new
		res.Status = MergeStatusCreated
		return res, nil
	}

	baseStr := string(base)
	currStr := string(current)
	newStr := string(new)

	// 2. Calculate Patch: Base -> New (What did the generator change?)
	// We want to apply Generator changes onto User's current file.
	patches := m.engine.PatchMake(baseStr, newStr)

	if len(patches) == 0 {
		// No changes from generator
		res.Content = current
		return res, nil
	}

	// 3. Apply Patch to Current
	mergedStr, results := m.engine.PatchApply(patches, currStr)

	// 4. Check for conflicts
	// PatchApply returns a bool array indicating success of each patch.
	// If any patch failed to apply, it means the context didn't match
	// (likely due to user edits in the same region).
	hasFailures := false
	for _, success := range results {
		if !success {
			hasFailures = true
			break
		}
	}

	if hasFailures {
		res.Status = MergeStatusConflict
		res.HasConflicts = true

		// For MVP, we return the partially merged content.
		// Ideally, we would inject conflict markers here.
		// Since diffmatchpatch doesn't natively support git-style conflict markers easily,
		// we will return the failed merge but flag it.
		// A more advanced implementation would reconstruct the markers.

		// Fallback strategy for conflicts:
		// To be safe, we often output the NEW content but maybe back up CURRENT?
		// Or we output the merged content with "fuzz".
		res.Content = []byte(mergedStr)

		// TODO: Implement proper conflict marker injection (<<<< ==== >>>>)
		// For now, we mark it as conflict so the Engine can warn the user.
	} else {
		res.Content = []byte(mergedStr)
	}

	return res, nil
}
