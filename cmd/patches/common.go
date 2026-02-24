package patches

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/lockfile"
	internalPatches "github.com/speakeasy-api/speakeasy/internal/patches"
)

// loadLockFile resolves the directory and loads the gen.lock file.
func loadLockFile(dir string) (string, *config.LockFile, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve directory: %w", err)
	}

	cfg, err := config.Load(absDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load config from %s: %w", absDir, err)
	}
	if cfg.LockFile == nil {
		return "", nil, fmt.Errorf("no gen.lock found in %s", absDir)
	}

	return absDir, cfg.LockFile, nil
}

// loadTrackedFile loads the lockfile, looks up a specific tracked file, and
// verifies it has a pristine git object. Returns everything needed to operate
// on a single tracked file.
func loadTrackedFile(dir, file string) (string, lockfile.TrackedFile, internalPatches.GitRepository, error) {
	absDir, lf, err := loadLockFile(dir)
	if err != nil {
		return "", lockfile.TrackedFile{}, nil, err
	}

	tracked, ok := lf.TrackedFiles.Get(file)
	if !ok {
		return "", lockfile.TrackedFile{}, nil, fmt.Errorf("file %q is not tracked in gen.lock", file)
	}

	if tracked.PristineGitObject == "" {
		return "", lockfile.TrackedFile{}, nil, fmt.Errorf("file %q has no pristine git object recorded", file)
	}

	gitRepo, err := internalPatches.OpenGitRepository(absDir)
	if err != nil {
		return "", lockfile.TrackedFile{}, nil, err
	}

	return absDir, tracked, gitRepo, nil
}

// fileMatchesPristine checks if the file on disk is identical to its pristine version.
func fileMatchesPristine(dir, filePath string, pristineContent []byte) (bool, error) {
	current, err := os.ReadFile(filepath.Join(dir, filePath))
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	return bytes.Equal(current, pristineContent), nil
}

// restoreFileToPristine writes the pristine content of a tracked file to disk,
// preserving existing file permissions.
func restoreFileToPristine(dir, filePath string, pristineContent []byte) error {
	fullPath := filepath.Join(dir, filePath)

	perm := os.FileMode(0o644)
	if info, err := os.Stat(fullPath); err == nil {
		perm = info.Mode().Perm()
	}

	if err := os.WriteFile(fullPath, pristineContent, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}
