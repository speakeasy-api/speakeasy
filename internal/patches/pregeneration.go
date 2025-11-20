package patches

import (
	"os"
	"path/filepath"

	config "github.com/speakeasy-api/sdk-gen-config"
)

// DetectFileChanges scans the output directory and updates the lockfile's TrackedFiles
// with Deleted and MovedTo fields based on the current state of files on disk.
// This should be called before generation when persistentEdits is enabled.
//
// The function:
// 1. Scans for @generated-id headers in all files
// 2. Compares UUIDs to the lockfile's TrackedFiles
// 3. Marks files as Deleted if their UUID is no longer on disk
// 4. Sets MovedTo if a file's UUID is found at a different path
func DetectFileChanges(outDir string, lockFile *config.LockFile) error {
	if lockFile == nil || lockFile.TrackedFiles == nil {
		return nil
	}

	// Scan for @generated-id headers on disk
	scanner := NewScanner(outDir)
	scanResult, err := scanner.Scan()
	if err != nil {
		return err
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
			tracked.Deleted = true
			tracked.MovedTo = ""
			lockFile.TrackedFiles.Set(path, tracked)
		} else if uuidFoundOnDisk && currentPath != path {
			// File was moved - UUID found at different path
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
		}
	}

	return nil
}
