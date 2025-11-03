package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type FindOrphanedFilesFlags struct {
	Directory string `json:"dir"`
	Verbose   bool   `json:"verbose"`
	SkipMd    bool   `json:"skip-md"`
}

var debugCmd = &model.CommandGroup{
	Usage:  "debug",
	Short:  "Debug commands for troubleshooting",
	Hidden: true,
	Commands: []model.Command{
		&model.ExecutableCommand[FindOrphanedFilesFlags]{
			Usage:  "find-orphaned-files",
			Short:  "Find generated files that are not tracked in gen.lock",
			Long:   "Find generated files that exist on disk but are not listed in gen.lock files. Files must contain 'speakeasy.com' to be considered orphaned.",
			Hidden: true,
			Run:    findOrphanedFiles,
			Flags: []flag.Flag{
				flag.StringFlag{
					Name:         "dir",
					Shorthand:    "d",
					Description:  "directory to check for generated files",
					DefaultValue: ".",
				},
				flag.BooleanFlag{
					Name:        "verbose",
					Shorthand:   "v",
					Description: "show detailed output",
				},
				flag.BooleanFlag{
					Name:         "skip-md",
					Description:  "skip markdown files",
					DefaultValue: true,
				},
			},
		},
	},
}

func findOrphanedFiles(ctx context.Context, flags FindOrphanedFilesFlags) error {
	logger := log.From(ctx)
	dir := flags.Directory
	if dir == "" {
		dir = "."
	}

	// Search for gen.lock files recursively
	genLockPaths, err := findGenLockFiles(dir)
	if err != nil {
		if flags.Verbose {
			logger.Errorf("error searching for gen.lock files: %v", err)
		}
		return err
	}
	if len(genLockPaths) == 0 {
		if flags.Verbose {
			logger.Errorf("no gen.lock files found in %s", dir)
		}
		return fmt.Errorf("no gen.lock files found in %s", dir)
	}
	if flags.Verbose {
		logger.Infof("Found %d gen.lock file(s):", len(genLockPaths))
		for _, p := range genLockPaths {
			relPath, _ := filepath.Rel(dir, p)
			logger.Infof("  - %s", relPath)
		}
		logger.Info("")
	}

	totalFiles := 0
	allFilesInGenLock := make(map[string]bool)

	// Process each gen.lock file
	for _, lockPath := range genLockPaths {
		files, err := parseGeneratedFiles(lockPath)
		if err != nil {
			if flags.Verbose {
				relPath, _ := filepath.Rel(dir, lockPath)
				logger.Warnf("warning: failed to parse %s: %v", relPath, err)
			}
			continue
		}

		// Get the directory containing this gen.lock file
		lockDir := filepath.Dir(lockPath)
		if filepath.Base(lockDir) == ".speakeasy" {
			lockDir = filepath.Dir(lockDir)
		}

		totalFiles += len(files)

		// Add files to the global map, making paths relative to the base dir
		for _, f := range files {
			fullPath := filepath.Join(lockDir, f)
			relPath, err := filepath.Rel(dir, fullPath)
			if err != nil {
				allFilesInGenLock[filepath.Clean(f)] = true
				continue
			}
			allFilesInGenLock[filepath.Clean(relPath)] = true
		}
	}

	// Check for orphaned files (files that exist but are not in gen.lock)
	allFiles, err := walkGeneratedDirs(dir, flags.SkipMd)
	if err != nil {
		if flags.Verbose {
			logger.Errorf("error walking directories: %v", err)
		}
		return err
	}

	var orphanedFiles []string
	for _, file := range allFiles {
		clean := filepath.Clean(file)
		if allFilesInGenLock[clean] {
			continue
		}

		// Skip files with "hooks" in the path
		if strings.Contains(clean, "hooks") {
			continue
		}

		if flags.SkipMd && strings.HasSuffix(clean, ".md") {
			continue
		}

		// Check if file contains "speakeasy.com" before flagging as orphaned
		fullPath := filepath.Join(dir, file)
		if !containsSpeakeasy(fullPath) {
			continue
		}

		orphanedFiles = append(orphanedFiles, clean)
	}

	if flags.Verbose {
		logger.Infof("\nSummary:")
		logger.Infof("Files in gen.lock: %d", totalFiles)
		logger.Infof("Orphaned files: %d", len(orphanedFiles))
		if len(orphanedFiles) > 0 {
			logger.Info("\nOrphaned files:")
			for _, file := range orphanedFiles {
				logger.Infof("  - %s", file)
			}
		}
	} else {
		// Non-verbose mode: only output orphaned files
		if len(orphanedFiles) > 0 {
			logger.Info("Orphaned files:")
			for _, file := range orphanedFiles {
				logger.Info(file)
			}
		} else {
			logger.Info("No orphaned files found")
		}
	}

	if len(orphanedFiles) > 0 {
		return fmt.Errorf("found %d orphaned files", len(orphanedFiles))
	}

	return nil
}

// genLockFile represents the structure of a gen.lock file
// We only need the generatedFiles field, so we use yaml.Node to avoid
// unmarshaling the entire file structure.
type genLockFile struct {
	GeneratedFiles []string `yaml:"generatedFiles"`
}

// parseGeneratedFiles reads a YAML file and extracts the entries under the
// top-level `generatedFiles:` key using the yaml library.
func parseGeneratedFiles(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lockFile genLockFile
	if err := yaml.Unmarshal(data, &lockFile); err != nil {
		return nil, fmt.Errorf("failed to parse gen.lock file: %w", err)
	}

	if len(lockFile.GeneratedFiles) == 0 {
		return nil, fmt.Errorf("no generatedFiles entries found in gen.lock")
	}

	return lockFile.GeneratedFiles, nil
}

// containsSpeakeasy checks if a file contains the string "speakeasy.com"
func containsSpeakeasy(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false // If we can't read the file, assume it doesn't contain the string
	}
	return strings.Contains(string(content), "speakeasy.com")
}

// findGenLockFiles recursively searches for gen.lock files in the given directory
func findGenLockFiles(baseDir string) ([]string, error) {
	var genLockPaths []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip common build artifacts and cache directories
		if info.IsDir() {
			baseName := filepath.Base(path)
			if baseName == "node_modules" || baseName == ".git" || baseName == "vendor" || baseName == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this is a gen.lock file
		if filepath.Base(path) == "gen.lock" {
			genLockPaths = append(genLockPaths, path)
		}
		return nil
	})
	return genLockPaths, err
}

// walkGeneratedDirs walks through all src directories and returns all file paths found
func walkGeneratedDirs(baseDir string, skipMdFiles bool) ([]string, error) {
	var allFiles []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			baseName := filepath.Base(path)

			// Define skip conditions for better readability
			isPackageDir := baseName == "node_modules" || baseName == "vendor"
			isBuildArtifact := baseName == "dist"
			isPythonCache := strings.HasPrefix(baseName, "__")
			isHiddenDir := strings.HasPrefix(baseName, ".") && baseName != "."
			isDocsDir := baseName == "docs"

			// Skip common build artifacts and cache directories
			if isPackageDir || isBuildArtifact || isPythonCache || isHiddenDir {
				return filepath.SkipDir
			}

			// Skip docs directory if skipMdFiles is enabled
			if skipMdFiles && isDocsDir {
				return filepath.SkipDir
			}
			return nil
		}

		baseName := filepath.Base(path)
		// Skip common build artifact files
		if strings.HasSuffix(baseName, ".pyc") || strings.HasSuffix(baseName, ".pyo") {
			return nil
		}

		// Make the path relative to baseDir for comparison with gen.lock
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		allFiles = append(allFiles, relPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return allFiles, nil
}

