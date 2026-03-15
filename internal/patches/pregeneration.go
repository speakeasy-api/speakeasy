package patches

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	generatorpatchfiles "github.com/speakeasy-api/openapi-generation/v2/pkg/patchfiles"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/prompts"
)

// PromptFunc is the function signature for prompting the user for custom code choices.
type PromptFunc func(summary string) (prompts.CustomCodeChoice, error)

type CaptureMode string

const (
	CaptureModePatchFiles CaptureMode = "patch_files"
	CaptureModeChangeset  CaptureMode = "changeset"
)

type CaptureRequiredError struct {
	Mode    CaptureMode
	Summary string
}

type PreparationResult struct {
	TrustedPatchInputs bool
}

func (e *CaptureRequiredError) Error() string {
	if e == nil {
		return ""
	}

	message := "generated SDK files contain unmanaged edits that are not represented by trusted patch state"
	command := "speakeasy patches capture"
	if e.Mode == CaptureModeChangeset {
		command = "speakeasy run --auto-yes"
	}
	if strings.TrimSpace(e.Summary) == "" {
		return message + "\n\nCapture them before generating with:\n\n  " + command
	}

	return fmt.Sprintf("%s:\n%s\n\nCapture them before generating with:\n\n  %s", message, e.Summary, command)
}

func IsCaptureRequired(err error) (*CaptureRequiredError, bool) {
	var captureErr *CaptureRequiredError
	if errors.As(err, &captureErr) {
		return captureErr, true
	}
	return nil, false
}

// FileChangeSummary contains a summary of detected file changes
type FileChangeSummary struct {
	Deleted  []string
	Moved    map[string]string // original path -> new path
	Modified []FileDiff
}

// GetFileChangeSummary extracts a summary of file changes from the lockfile
func GetFileChangeSummary(lockFile *config.LockFile) FileChangeSummary {
	summary := FileChangeSummary{
		Moved: make(map[string]string),
	}

	if lockFile == nil || lockFile.TrackedFiles == nil {
		return summary
	}

	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		if tracked.Deleted {
			summary.Deleted = append(summary.Deleted, path)
		} else if movedTo := GetMovedTo(tracked); movedTo != "" {
			summary.Moved[path] = movedTo
		}
	}

	return summary
}

// GetFileChangeSummaryWithDiffs extracts summary and computes diffs for modified files.
// This is the enhanced version that includes actual diff content for modified files.
func GetFileChangeSummaryWithDiffs(
	outDir string,
	lockFile *config.LockFile,
	modifiedPaths []string,
	gitRepo GitRepository,
) FileChangeSummary {
	summary := FileChangeSummary{
		Moved:    make(map[string]string),
		Modified: make([]FileDiff, 0, len(modifiedPaths)),
	}

	if lockFile == nil || lockFile.TrackedFiles == nil {
		return summary
	}

	// Handle deleted and moved (same as GetFileChangeSummary)
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		if tracked.Deleted {
			summary.Deleted = append(summary.Deleted, path)
		} else if movedTo := GetMovedTo(tracked); movedTo != "" {
			summary.Moved[path] = movedTo
		}
	}

	// Handle modified - compute diffs
	for _, path := range modifiedPaths {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		fd := ComputeFileDiff(outDir, path, tracked.PristineGitObject, gitRepo)
		summary.Modified = append(summary.Modified, fd)
	}

	return summary
}

// FormatSummary formats the change summary as a git-status-like output, truncated to maxLines.
// If showDiffs is true, includes truncated diff content for modified files.
func (s FileChangeSummary) FormatSummary(maxLines int, showDiffs bool) string {
	var lines []string

	for _, path := range s.Deleted {
		lines = append(lines, fmt.Sprintf("  D %s", path))
	}
	for from, to := range s.Moved {
		lines = append(lines, fmt.Sprintf("  R %s -> %s", from, to))
	}
	for _, fd := range s.Modified {
		if showDiffs && fd.Stats.Added+fd.Stats.Removed > 0 {
			lines = append(lines, fmt.Sprintf("  M %s (+%d/-%d)", fd.Path, fd.Stats.Added, fd.Stats.Removed))
			// Add truncated diff (max 10 lines per file)
			diffLines := strings.Split(strings.TrimSpace(fd.DiffText), "\n")
			maxDiffLines := 10
			for i, dl := range diffLines {
				if i >= maxDiffLines {
					lines = append(lines, fmt.Sprintf("      ... (%d more lines)", len(diffLines)-maxDiffLines))
					break
				}
				lines = append(lines, "      "+dl)
			}
		} else {
			lines = append(lines, fmt.Sprintf("  M %s", fd.Path))
		}
	}

	total := len(lines)
	if total > maxLines && maxLines > 0 {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("  ... and %d more", total-maxLines))
	}

	return strings.Join(lines, "\n")
}

// IsEmpty returns true if no changes were detected
func (s FileChangeSummary) IsEmpty() bool {
	return len(s.Deleted) == 0 && len(s.Moved) == 0 && len(s.Modified) == 0
}

// DetectFileChanges scans the output directory and updates the lockfile's TrackedFiles
// with Deleted markers and any previously-recorded move metadata.
// Returns:
//   - isDirty: true if any tracked file has been deleted or has a checksum change
//   - modifiedPaths: list of paths that have content modifications (checksum mismatch)
//   - error: any error encountered during scanning
//
// The function:
// 1. Scans for @generated-id headers in all files
// 2. Preserves explicit move metadata already present in gen.lock
// 3. Marks files as Deleted if their expected path is no longer on disk
// 4. Detects checksum changes for files in their expected location
func DetectFileChanges(outDir string, lockFile *config.LockFile) (bool, []string, error) {
	if lockFile == nil || lockFile.TrackedFiles == nil {
		return false, nil, nil
	}

	isDirty := false
	var modifiedPaths []string

	// Scan for @generated-id headers on disk
	scanner := NewScanner(outDir)
	scanResult, err := scanner.Scan()
	if err != nil {
		return false, nil, err
	}

	// Process each tracked file
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}

		movedTo := GetMovedTo(tracked)
		checksumPath := path
		fileExists := false

		if movedTo != "" {
			movedToFullPath := filepath.Join(outDir, filepath.FromSlash(movedTo))
			if _, err := os.Stat(movedToFullPath); err == nil {
				fileExists = true
				checksumPath = movedTo
			} else {
				fullPath := filepath.Join(outDir, path)
				if _, err := os.Stat(fullPath); err == nil {
					fileExists = true
					tracked.Deleted = false
					SetMovedTo(&tracked, "")
					lockFile.TrackedFiles.Set(path, tracked)
					checksumPath = path
					movedTo = ""
				}
			}
		} else {
			fullPath := filepath.Join(outDir, path)
			_, fileErr := os.Stat(fullPath)
			fileExists = fileErr == nil
		}

		switch {
		case !fileExists:
			isDirty = true
			tracked.Deleted = true
			SetMovedTo(&tracked, "")
			lockFile.TrackedFiles.Set(path, tracked)
		default:
			// File is in its expected location, clear any stale delete marker.
			if tracked.Deleted {
				tracked.Deleted = false
				lockFile.TrackedFiles.Set(path, tracked)
			}

			// If an explicit move exists but the same generated ID is no longer at the destination,
			// treat it as stale and fall back to the original path on the next run.
			if movedTo != "" && tracked.ID != "" {
				if destinationID, ok := scanResult.PathToUUID[movedTo]; ok && destinationID != tracked.ID {
					SetMovedTo(&tracked, "")
					lockFile.TrackedFiles.Set(path, tracked)
					checksumPath = path
				}
			}

			// Check for content modification via checksum
			if tracked.LastWriteChecksum != "" {
				currentChecksum, err := lockfile.ComputeFileChecksum(os.DirFS(outDir), checksumPath)
				if err == nil && currentChecksum != tracked.LastWriteChecksum {
					isDirty = true
					modifiedPaths = append(modifiedPaths, path)
				}
			}
		}
	}

	return isDirty, modifiedPaths, nil
}

// PrepareForGeneration detects custom code changes and optionally prompts the user.
// This should be called before SDK generation when git is available.
//
// Parameters:
//   - outDir: the output directory for generation
//   - autoYes: if true, skip prompting (used in CI/workflow mode)
//   - promptFunc: function to prompt user for custom code choice (nil to skip prompting)
//   - warnFunc: function to log warnings
//
// Returns error only for fatal errors. If config or lockfile is missing, returns nil (opportunistic execution).
func PrepareForGeneration(outDir string, autoYes bool, patchCapture bool, changesetCapture bool, promptFunc PromptFunc, warnFunc func(format string, args ...any)) (PreparationResult, error) {
	cfg, err := config.Load(outDir)
	if err != nil {
		return PreparationResult{}, nil //nolint:nilerr // Ignore error
	}

	if cfg.LockFile == nil || cfg.Config == nil {
		return PreparationResult{}, nil
	}

	persistentEdits := cfg.Config.Generation.PersistentEdits
	changesetMode := usesChangesetVersioning(cfg.Config)
	result := PreparationResult{
		TrustedPatchInputs: persistentEdits.UsesPatchFiles() && !patchCapture && !changesetMode,
	}

	if persistentEdits.IsEnabled() {
		// Persistent edits enabled - detect changes and save lockfile
		isDirty, modifiedPaths, err := DetectFileChanges(outDir, cfg.LockFile)
		if err != nil {
			warnFunc("Failed to detect file changes: %v", err)
		} else {
			gitRepo, _ := OpenGitRepository(outDir)
			if changesetMode && isDirty && !changesetCapture {
				summary := GetFileChangeSummaryWithDiffs(outDir, cfg.LockFile, modifiedPaths, gitRepo)
				if summary.IsEmpty() {
					return result, nil
				}
				return result, &CaptureRequiredError{
					Mode:    CaptureModeChangeset,
					Summary: summary.FormatSummary(20, gitRepo != nil),
				}
			}
			if cfg.Config.Generation.PersistentEdits.UsesPatchFiles() && isDirty && !patchCapture && !changesetMode {
				modifiedPaths = filterCapturedPatchPaths(outDir, cfg.LockFile, modifiedPaths, gitRepo)
				summary := GetFileChangeSummaryWithDiffs(outDir, cfg.LockFile, modifiedPaths, gitRepo)
				if summary.IsEmpty() {
					return result, nil
				}
				return result, &CaptureRequiredError{
					Mode:    CaptureModePatchFiles,
					Summary: summary.FormatSummary(20, gitRepo != nil),
				}
			}
			if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
				warnFunc("Failed to save lockfile with file change markers: %v", err)
			}
		}
	} else if !persistentEdits.IsNever() && !env.IsCI() {
		// Not enabled and not "never" - check for dirty files and prompt
		isDirty, modifiedPaths, err := DetectFileChanges(outDir, cfg.LockFile)
		if err != nil {
			warnFunc("Failed to detect file changes: %v", err)
		} else if isDirty && !autoYes && promptFunc != nil {
			// Initialize git repo for reading blobs (for diff display)
			gitRepo, _ := OpenGitRepository(outDir)

			// Get summary with diffs for display
			summary := GetFileChangeSummaryWithDiffs(outDir, cfg.LockFile, modifiedPaths, gitRepo)
			summaryText := summary.FormatSummary(20, gitRepo != nil /* showDiffs only if git available */)

			choice, err := promptFunc(summaryText)
			if err != nil {
				warnFunc("Failed to prompt for custom code: %v", err)
			} else {
				switch choice {
				case prompts.CustomCodeChoiceYes:
					// Enable persistent edits in config
					enabled := config.PersistentEditsEnabledTrue
					cfg.Config.Generation.PersistentEdits.Enabled = &enabled
					if err := config.SaveConfig(outDir, cfg.Config); err != nil {
						warnFunc("Failed to save config with persistent edits enabled: %v", err)
					}
				case prompts.CustomCodeChoiceDontAskAgain:
					// Set to never in config
					never := config.PersistentEditsEnabledNever
					cfg.Config.Generation.PersistentEdits.Enabled = &never
					if err := config.SaveConfig(outDir, cfg.Config); err != nil {
						warnFunc("Failed to save config with persistent edits disabled: %v", err)
					}
				case prompts.CustomCodeChoiceNo:
					// Continue without changes
				}
			}
		}
	}

	return result, nil
}

func filterCapturedPatchPaths(outDir string, lockFile *config.LockFile, modifiedPaths []string, gitRepo GitRepository) []string {
	if lockFile == nil || lockFile.TrackedFiles == nil || gitRepo == nil || gitRepo.IsNil() {
		return modifiedPaths
	}

	filtered := make([]string, 0, len(modifiedPaths))
	for _, path := range modifiedPaths {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok || !pathMatchesTrustedPatchState(outDir, path, tracked, gitRepo) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

func pathMatchesTrustedPatchState(outDir, path string, tracked lockfile.TrackedFile, gitRepo GitRepository) bool {
	pristineHash := strings.TrimSpace(tracked.PristineGitObject)
	if pristineHash == "" {
		return false
	}

	pristine, err := gitRepo.GetBlob(pristineHash)
	if err != nil {
		return false
	}

	expected, used, err := generatorpatchfiles.Apply(outDir, path, pristine)
	if err != nil || !used {
		return false
	}

	actualPath := path
	if movedTo := GetMovedTo(tracked); movedTo != "" {
		fullMovedPath := filepath.Join(outDir, filepath.FromSlash(movedTo))
		if _, err := os.Stat(fullMovedPath); err == nil {
			actualPath = movedTo
		}
	}

	current, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(actualPath)))
	if err != nil {
		return false
	}

	return bytes.Equal(expected, current)
}

func usesChangesetVersioning(cfg *config.Configuration) bool {
	if cfg == nil {
		return false
	}

	if strings.EqualFold(string(cfg.Generation.VersioningStrategy), "changeset") {
		return true
	}

	if raw, ok := cfg.Generation.AdditionalProperties["versionStrategy"]; ok {
		if value, ok := raw.(string); ok && strings.EqualFold(value, "changeset") {
			return true
		}
	}

	return false
}
