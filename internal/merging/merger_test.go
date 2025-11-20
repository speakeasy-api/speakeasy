package merging

import (
	"strings"
	"testing"
)

func TestTextMerger_Merge_CleanMerge(t *testing.T) {
	// User changed line 2, generator changed line 4
	// Non-overlapping changes should merge cleanly
	base := []byte("line 1\nline 2\nline 3\nline 4\n")
	current := []byte("line 1\nline 2 changed by user\nline 3\nline 4\n")
	new := []byte("line 1\nline 2\nline 3\nline 4 changed by generator\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusClean {
		t.Errorf("Expected MergeStatusClean, got %v", result.Status)
	}

	if result.HasConflicts {
		t.Error("Expected no conflicts")
	}

	content := string(result.Content)
	if !strings.Contains(content, "line 2 changed by user") {
		t.Error("User's change to line 2 was lost")
	}
	if !strings.Contains(content, "line 4 changed by generator") {
		t.Error("Generator's change to line 4 was lost")
	}
}

func TestTextMerger_Merge_ConflictMarkers(t *testing.T) {
	// Both user and generator changed line 2 - should create conflict
	base := []byte("line 1\nline 2\nline 3\n")
	current := []byte("line 1\nline 2 changed by user\nline 3\n")
	new := []byte("line 1\nline 2 changed by generator\nline 3\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusConflict {
		t.Errorf("Expected MergeStatusConflict, got %v", result.Status)
	}

	if !result.HasConflicts {
		t.Error("Expected HasConflicts to be true")
	}

	content := string(result.Content)

	// Verify conflict markers are present
	if !strings.Contains(content, "<<<<<<<") {
		t.Error("Missing conflict start marker")
	}
	if !strings.Contains(content, "=======") {
		t.Error("Missing conflict separator")
	}
	if !strings.Contains(content, ">>>>>>>") {
		t.Error("Missing conflict end marker")
	}

	// Verify both versions are present in the conflict
	if !strings.Contains(content, "changed by user") {
		t.Error("User's version missing from conflict markers")
	}
	if !strings.Contains(content, "changed by generator") {
		t.Error("Generator's version missing from conflict markers")
	}

	// Verify conflicts were parsed
	if len(result.Conflicts) == 0 {
		t.Error("Expected conflicts to be parsed and recorded")
	}
}

func TestTextMerger_Merge_FastForward_IdenticalContent(t *testing.T) {
	// Current and new are identical - no merge needed
	base := []byte("line 1\nline 2\n")
	current := []byte("line 1\nline 2\nline 3\n")
	new := []byte("line 1\nline 2\nline 3\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusFastForward {
		t.Errorf("Expected MergeStatusFastForward, got %v", result.Status)
	}

	if string(result.Content) != string(current) {
		t.Error("Content should match current")
	}
}

func TestTextMerger_Merge_FastForward_UserNoChanges(t *testing.T) {
	// User made no changes, generator made changes
	// Should fast-forward to new content
	base := []byte("line 1\nline 2\n")
	current := []byte("line 1\nline 2\n") // Same as base
	new := []byte("line 1\nline 2\nline 3\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusFastForward {
		t.Errorf("Expected MergeStatusFastForward, got %v", result.Status)
	}

	if string(result.Content) != string(new) {
		t.Error("Content should match new (generator's version)")
	}
}

func TestTextMerger_Merge_NoGeneratorChanges(t *testing.T) {
	// Generator made no changes, user made changes
	// Should keep user's version
	base := []byte("line 1\nline 2\n")
	current := []byte("line 1\nline 2\nuser's addition\n")
	new := []byte("line 1\nline 2\n") // Same as base

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusClean {
		t.Errorf("Expected MergeStatusClean, got %v", result.Status)
	}

	if string(result.Content) != string(current) {
		t.Error("Content should match current (user's version)")
	}
}

func TestTextMerger_Merge_NewFile(t *testing.T) {
	// No base means this is a new file
	var base []byte = nil
	current := []byte("user created this\n")
	new := []byte("generator created this\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusCreated {
		t.Errorf("Expected MergeStatusCreated, got %v", result.Status)
	}

	// For new files, generator wins
	if string(result.Content) != string(new) {
		t.Error("Content should match new (generator is authoritative for new files)")
	}
}

func TestTextMerger_Merge_EmptyBase(t *testing.T) {
	// Empty base is treated same as nil base
	base := []byte("")
	current := []byte("existing content\n")
	new := []byte("generator content\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusCreated {
		t.Errorf("Expected MergeStatusCreated, got %v", result.Status)
	}
}

func TestTextMerger_Merge_MultipleConflicts(t *testing.T) {
	// Multiple conflicting regions
	base := []byte("line 1\nline 2\nline 3\nline 4\n")
	current := []byte("line 1 user\nline 2\nline 3 user\nline 4\n")
	new := []byte("line 1 gen\nline 2\nline 3 gen\nline 4\n")

	merger := NewTextMerger()
	result, err := merger.Merge(base, current, new)

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if result.Status != MergeStatusConflict {
		t.Errorf("Expected MergeStatusConflict, got %v", result.Status)
	}

	if !result.HasConflicts {
		t.Error("Expected HasConflicts to be true")
	}

	// Should have multiple conflicts parsed
	if len(result.Conflicts) < 2 {
		t.Errorf("Expected at least 2 conflicts, got %d", len(result.Conflicts))
	}
}

func TestParseConflictMarkers(t *testing.T) {
	content := `line 1
<<<<<<< CURRENT
user version
=======
generator version
>>>>>>> NEW
line 2
<<<<<<< CURRENT
another user change
=======
another generator change
>>>>>>> NEW
line 3`

	conflicts := parseConflictMarkers(content)

	if len(conflicts) != 2 {
		t.Errorf("Expected 2 conflicts, got %d", len(conflicts))
	}

	// Check first conflict
	if conflicts[0].StartLine != 2 {
		t.Errorf("First conflict start line = %d, want 2", conflicts[0].StartLine)
	}
	if conflicts[0].EndLine != 6 {
		t.Errorf("First conflict end line = %d, want 6", conflicts[0].EndLine)
	}

	// Check second conflict
	if conflicts[1].StartLine != 8 {
		t.Errorf("Second conflict start line = %d, want 8", conflicts[1].StartLine)
	}
	if conflicts[1].EndLine != 12 {
		t.Errorf("Second conflict end line = %d, want 12", conflicts[1].EndLine)
	}
}

func TestParseConflictMarkers_NoConflicts(t *testing.T) {
	content := "line 1\nline 2\nline 3\n"

	conflicts := parseConflictMarkers(content)

	if len(conflicts) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(conflicts))
	}
}
