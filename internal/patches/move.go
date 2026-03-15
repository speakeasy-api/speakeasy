package patches

import (
	"fmt"
	"os"
	"path/filepath"

	config "github.com/speakeasy-api/sdk-gen-config"
)

func RecordMove(outDir, fromPath, toPath string) error {
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", outDir, err)
	}

	if cfg.LockFile == nil {
		return fmt.Errorf("no gen.lock found in %s", outDir)
	}

	fromPath = normalizeTrackedPath(fromPath)
	toPath = normalizeTrackedPath(toPath)

	if fromPath == "" || toPath == "" {
		return fmt.Errorf("source and destination paths are required")
	}
	if fromPath == toPath {
		return fmt.Errorf("source and destination paths must be different")
	}

	tracked, ok := cfg.LockFile.TrackedFiles.Get(fromPath)
	if !ok {
		return fmt.Errorf("file %q is not tracked in gen.lock", fromPath)
	}

	destination := filepath.Join(outDir, filepath.FromSlash(toPath))
	info, err := os.Stat(destination)
	if err != nil {
		return fmt.Errorf("failed to stat destination %q: %w", toPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("destination %q is a directory", toPath)
	}

	if tracked.ID != "" {
		scanResult, err := NewScanner(outDir).Scan()
		if err != nil {
			return fmt.Errorf("failed to scan generated files: %w", err)
		}

		destinationID, ok := scanResult.PathToUUID[toPath]
		if !ok {
			return fmt.Errorf("destination %q does not contain a matching @generated-id header", toPath)
		}
		if destinationID != tracked.ID {
			return fmt.Errorf("destination %q has generated id %q, expected %q", toPath, destinationID, tracked.ID)
		}
	}

	tracked.Deleted = false
	SetMovedTo(&tracked, toPath)
	cfg.LockFile.TrackedFiles.Set(fromPath, tracked)

	if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
		return fmt.Errorf("failed to save gen.lock: %w", err)
	}

	return nil
}

func normalizeTrackedPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
