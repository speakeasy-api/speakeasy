package patches

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/prompts"
)

// PromptFunc is the function signature for prompting the user for custom code choices.
type PromptFunc func(summary string) (prompts.CustomCodeChoice, error)

// FileChangeSummary contains a summary of detected file changes
type FileChangeSummary struct {
	Deleted  []string
	Moved    map[string]string // original path -> new path
	Modified []string
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
		} else if tracked.MovedTo != "" {
			summary.Moved[path] = tracked.MovedTo
		}
	}

	return summary
}

// FormatSummary formats the change summary as a git-status-like output, truncated to maxLines
func (s FileChangeSummary) FormatSummary(maxLines int) string {
	var lines []string

	for _, path := range s.Deleted {
		lines = append(lines, fmt.Sprintf("  D %s", path))
	}
	for from, to := range s.Moved {
		lines = append(lines, fmt.Sprintf("  R %s -> %s", from, to))
	}
	for _, path := range s.Modified {
		lines = append(lines, fmt.Sprintf("  M %s", path))
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
// with Deleted and MovedTo fields based on the current state of files on disk.
// Returns isDirty (true if any tracked file has been deleted, moved, or has a checksum change)
// and any error encountered.
//
// The function:
// 1. Scans for @generated-id headers in all files
// 2. Compares UUIDs to the lockfile's TrackedFiles
// 3. Marks files as Deleted if their UUID is no longer on disk
// 4. Sets MovedTo if a file's UUID is found at a different path
// 5. Detects checksum changes for files in their expected location
func DetectFileChanges(outDir string, lockFile *config.LockFile) (bool, error) {
	if lockFile == nil || lockFile.TrackedFiles == nil {
		return false, nil
	}

	isDirty := false

	// Scan for @generated-id headers on disk
	scanner := NewScanner(outDir)
	scanResult, err := scanner.Scan()
	if err != nil {
		return false, err
	}

	// Build a map of UUID -> original path from lockfile
	uuidToOriginalPath := make(map[string]string)
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok || tracked.ID == "" {
			continue
		}
		uuidToOriginalPath[tracked.ID] = path
	}

	// Process each tracked file
	for path := range lockFile.TrackedFiles.Keys() {
		tracked, ok := lockFile.TrackedFiles.Get(path)
		if !ok {
			continue
		}

		// Skip files without UUIDs (can't track moves without identity)
		if tracked.ID == "" {
			continue
		}

		// Check if file exists at its original path
		fullPath := filepath.Join(outDir, path)
		_, fileErr := os.Stat(fullPath)
		fileExists := fileErr == nil

		// Check if UUID is found at a different path
		currentPath, uuidFoundOnDisk := scanResult.UUIDToPath[tracked.ID]

		if !uuidFoundOnDisk && !fileExists {
			// File was deleted - UUID not found anywhere on disk
			isDirty = true
			tracked.Deleted = true
			tracked.MovedTo = ""
			lockFile.TrackedFiles.Set(path, tracked)
		} else if uuidFoundOnDisk && currentPath != path {
			// File was moved - UUID found at different path
			isDirty = true
			tracked.Deleted = false
			tracked.MovedTo = currentPath
			lockFile.TrackedFiles.Set(path, tracked)
		} else {
			// File is in its expected location, clear any stale move/delete markers
			if tracked.Deleted || tracked.MovedTo != "" {
				tracked.Deleted = false
				tracked.MovedTo = ""
				lockFile.TrackedFiles.Set(path, tracked)
			}

			// Check for content modification via checksum
			if tracked.LastWriteChecksum != "" && fileExists {
				currentChecksum, err := computeFileChecksum(fullPath)
				if err == nil && currentChecksum != tracked.LastWriteChecksum {
					isDirty = true
				}
			}
		}
	}

	return isDirty, nil
}

// computeFileChecksum calculates the SHA-1 checksum of a file
func computeFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// PrepareForGeneration detects custom code changes and optionally prompts the user.
// This should be called before SDK generation when git is available.
//
// Parameters:
//   - outDir: the output directory for generation
//   - autoYes: if true, skip prompting (used in CI/workflow mode)
//   - promptFunc: function to prompt user for custom code choice (can be nil if autoYes=true)
//   - warnFunc: function to log warnings
//
// Returns error only for fatal errors. If config or lockfile is missing, returns nil (opportunistic execution).
func PrepareForGeneration(outDir string, autoYes bool, promptFunc PromptFunc, warnFunc func(format string, args ...any)) error {
	cfg, err := config.Load(outDir)
	if err != nil {
		// If we can't load config, skip persistent edits check (matches original behavior)
		return nil
	}

	if cfg.LockFile == nil || cfg.Config == nil {
		return nil
	}

	persistentEdits := cfg.Config.Generation.PersistentEdits

	if persistentEdits.IsEnabled() {
		// Persistent edits enabled - detect changes and save lockfile
		if _, err := DetectFileChanges(outDir, cfg.LockFile); err != nil {
			warnFunc("Failed to detect file changes: %v", err)
		} else {
			if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
				warnFunc("Failed to save lockfile with file change markers: %v", err)
			}
		}
	} else if !persistentEdits.IsNever() {
		// Not enabled and not "never" - check for dirty files and prompt
		isDirty, err := DetectFileChanges(outDir, cfg.LockFile)
		if err != nil {
			warnFunc("Failed to detect file changes: %v", err)
		} else if isDirty && !autoYes {
			// Get summary for display
			summary := GetFileChangeSummary(cfg.LockFile)
			summaryText := summary.FormatSummary(10)

			choice, err := promptFunc(summaryText)
			if err != nil {
				warnFunc("Failed to prompt for custom code: %v", err)
			} else {
				switch choice {
				case prompts.CustomCodeChoiceYes:
					// Enable persistent edits in config
					if cfg.Config.Generation.PersistentEdits == nil {
						cfg.Config.Generation.PersistentEdits = &config.PersistentEdits{}
					}
					enabled := config.PersistentEditsEnabledTrue
					cfg.Config.Generation.PersistentEdits.Enabled = &enabled
					if err := config.SaveConfig(outDir, cfg.Config); err != nil {
						warnFunc("Failed to save config with persistent edits enabled: %v", err)
					}
				case prompts.CustomCodeChoiceDontAskAgain:
					// Set to never in config
					if cfg.Config.Generation.PersistentEdits == nil {
						cfg.Config.Generation.PersistentEdits = &config.PersistentEdits{}
					}
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

		// Save lockfile with markers regardless
		if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
			warnFunc("Failed to save lockfile with file change markers: %v", err)
		}
	}

	return nil
}
