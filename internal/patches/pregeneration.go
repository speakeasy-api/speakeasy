package patches

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/prompts"
)

// PromptFunc is the function signature for prompting the user for custom code choices.
type PromptFunc func(summary string) (prompts.CustomCodeChoice, error)

// FileChangeSummary contains a summary of detected file changes
type FileChangeSummary struct {
	Deleted  []string
	Modified []FileDiff
}

// GetFileChangeSummary extracts a summary of file changes from the lockfile
func GetFileChangeSummary(lockFile *config.LockFile) FileChangeSummary {
	summary := FileChangeSummary{}

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
		Modified: make([]FileDiff, 0, len(modifiedPaths)),
	}

	if lockFile == nil || lockFile.TrackedFiles == nil {
		return summary
	}

	// Handle deleted (same as GetFileChangeSummary)
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}
		if tracked.Deleted {
			summary.Deleted = append(summary.Deleted, path)
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
	return len(s.Deleted) == 0 && len(s.Modified) == 0
}

// DetectFileChanges checks the output directory and updates the lockfile's TrackedFiles
// with Deleted fields based on the current state of files on disk.
// Returns:
//   - isDirty: true if any tracked file has been deleted or has a checksum change
//   - modifiedPaths: list of paths that have content modifications (checksum mismatch)
//   - error: any error encountered during scanning
//
// The function:
// 1. Checks if each tracked file still exists on disk
// 2. Marks files as Deleted if they no longer exist
// 3. Detects checksum changes for files in their expected location
func DetectFileChanges(outDir string, lockFile *config.LockFile) (bool, []string, error) {
	if lockFile == nil || lockFile.TrackedFiles == nil {
		return false, nil, nil
	}

	isDirty := false
	var modifiedPaths []string

	// Process each tracked file
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}

		// Check if file exists at its path
		fullPath := filepath.Join(outDir, path)
		_, fileErr := os.Stat(fullPath)
		fileExists := fileErr == nil

		if !fileExists {
			// File was deleted
			isDirty = true
			tracked.Deleted = true
			lockFile.TrackedFiles.Set(path, tracked)
		} else {
			// File is in its expected location, clear any stale delete marker
			if tracked.Deleted {
				tracked.Deleted = false
				lockFile.TrackedFiles.Set(path, tracked)
			}

			// Check for content modification via checksum
			if tracked.LastWriteChecksum != "" {
				currentChecksum, err := lockfile.ComputeFileChecksum(os.DirFS(outDir), path)
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
func PrepareForGeneration(outDir string, autoYes bool, promptFunc PromptFunc, warnFunc func(format string, args ...any)) error {
	cfg, err := config.Load(outDir)
	if err != nil {
		return nil //nolint:nilerr // Ignore error
	}

	if cfg.LockFile == nil || cfg.Config == nil {
		return nil
	}

	persistentEdits := cfg.Config.Generation.PersistentEdits

	if persistentEdits.IsEnabled() {
		// Persistent edits enabled - detect changes and save lockfile
		if _, _, err := DetectFileChanges(outDir, cfg.LockFile); err != nil {
			warnFunc("Failed to detect file changes: %v", err)
		} else {
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

	return nil
}
