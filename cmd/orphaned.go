package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type OrphanedFilesFlags struct {
	Directory   string `json:"directory"`
	Verbose     bool   `json:"verbose"`
	SkipMdFiles bool   `json:"skip-md"`
	Delete      bool   `json:"delete"`
}

var orphanedFilesCmd = &model.ExecutableCommand[OrphanedFilesFlags]{
	Usage:  "orphaned",
	Short:  "Find orphaned files in SDK (for CS troubleshooting)",
	Long:   "Find files that contain 'speakeasy.com' but are not tracked in gen.lock. Hidden command for Customer Success troubleshooting.",
	Hidden: true,
	Run:    runOrphanedFiles,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "directory",
			Shorthand:    "d",
			Description:  "directory to check for generated files (default: current directory)",
			DefaultValue: ".",
		},
		flag.BooleanFlag{
			Name:         "verbose",
			Shorthand:    "v",
			Description:  "show detailed output",
			DefaultValue: false,
		},
		flag.BooleanFlag{
			Name:         "skip-md",
			Description:  "skip markdown files",
			DefaultValue: true,
		},
		flag.BooleanFlag{
			Name:         "delete",
			Description:  "delete orphaned files (use with caution)",
			DefaultValue: false,
		},
	},
}

func runOrphanedFiles(ctx context.Context, flags OrphanedFilesFlags) error {
	logger := log.From(ctx)

	// Search for gen.lock files recursively
	genLockPaths, err := findGenLockFiles(flags.Directory)
	if err != nil {
		if flags.Verbose {
			logger.Errorf("error searching for gen.lock files: %v", err)
		}
		return fmt.Errorf("failed to search for gen.lock files: %w", err)
	}

	if len(genLockPaths) == 0 {
		if flags.Verbose {
			logger.Warnf("no gen.lock files found in %s", flags.Directory)
		}
		return fmt.Errorf("no gen.lock files found in %s", flags.Directory)
	}

	if flags.Verbose {
		logger.Infof("Found %d gen.lock file(s):", len(genLockPaths))
		for _, p := range genLockPaths {
			relPath, _ := filepath.Rel(flags.Directory, p)
			logger.Infof("  - %s", relPath)
		}
		logger.Println("")
	}

	totalFiles := 0
	allFilesInGenLock := make(map[string]bool)

	// Process each gen.lock file
	for _, lockPath := range genLockPaths {
		files, err := parseGeneratedFiles(lockPath)
		if err != nil {
			if flags.Verbose {
				relPath, _ := filepath.Rel(flags.Directory, lockPath)
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
			relPath, err := filepath.Rel(flags.Directory, fullPath)
			if err != nil {
				allFilesInGenLock[filepath.Clean(f)] = true
				continue
			}
			allFilesInGenLock[filepath.Clean(relPath)] = true
		}
	}

	// Check for orphaned files (files that exist but are not in gen.lock)
	allFiles, err := walkGeneratedDirs(flags.Directory, flags.SkipMdFiles)
	if err != nil {
		if flags.Verbose {
			logger.Errorf("error walking directories: %v", err)
		}
		return fmt.Errorf("failed to walk directories: %w", err)
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

		if flags.SkipMdFiles && strings.HasSuffix(clean, ".md") {
			continue
		}

		// Check if file contains "speakeasy.com" before flagging as orphaned
		fullPath := filepath.Join(flags.Directory, file)
		if !containsSpeakeasy(fullPath) {
			continue
		}

		orphanedFiles = append(orphanedFiles, clean)
	}

	// Handle deletion if requested
	if flags.Delete && len(orphanedFiles) > 0 {
		logger.Println("Deleting orphaned files...")
		deletedCount := 0
		var deleteErrors []string
		dirsToCheck := make(map[string]bool)

		for _, file := range orphanedFiles {
			fullPath := filepath.Join(flags.Directory, file)
			if err := os.Remove(fullPath); err != nil {
				deleteErrors = append(deleteErrors, fmt.Sprintf("%s: %v", file, err))
				if flags.Verbose {
					logger.Warnf("  Failed to delete %s: %v", file, err)
				}
			} else {
				deletedCount++
				if flags.Verbose {
					logger.Infof("  Deleted: %s", file)
				}
				// Track parent directories for empty directory cleanup
				dir := filepath.Dir(fullPath)
				dirsToCheck[dir] = true
			}
		}

		logger.Printf("\nDeleted %d of %d orphaned files", deletedCount, len(orphanedFiles))
		if len(deleteErrors) > 0 {
			logger.Warnf("Failed to delete %d files:", len(deleteErrors))
			for _, errMsg := range deleteErrors {
				logger.Warnf("  %s", errMsg)
			}
		}

		// Clean up empty directories
		if deletedCount > 0 {
			emptyDirsRemoved := cleanupEmptyDirectories(flags.Directory, dirsToCheck, flags.Verbose, logger)
			if emptyDirsRemoved > 0 && flags.Verbose {
				logger.Printf("Removed %d empty directories", emptyDirsRemoved)
			}
			logger.Println("\n✓ Cleanup complete")
		}

		return nil
	}

	// Display results (when not deleting)
	if flags.Verbose {
		logger.Printf("\nSummary:")
		logger.Printf("Files in gen.lock: %d", totalFiles)
		logger.Printf("Orphaned files: %d", len(orphanedFiles))
		if len(orphanedFiles) > 0 {
			logger.Println("\nOrphaned files:")
			for _, file := range orphanedFiles {
				logger.Printf("  - %s", file)
			}
			logger.Println("\nTo delete these files, run with --delete flag")
		}
	} else {
		// Non-verbose mode: only output orphaned files
		if len(orphanedFiles) > 0 {
			logger.Println("Orphaned files:")
			for _, file := range orphanedFiles {
				logger.Printf("%s", file)
			}
			logger.Println("\nTo delete these files, run with --delete flag")
		} else {
			logger.Println("No orphaned files found")
		}
	}

	if len(orphanedFiles) > 0 {
		return fmt.Errorf("found %d orphaned files", len(orphanedFiles))
	}

	return nil
}

// parseGeneratedFiles scans a YAML file and extracts the entries under the
// top-level `generatedFiles:` key without relying on external YAML packages.
// It supports simple YAML lists of strings in the form:
//
// generatedFiles:
//   - path/one
//   - path/two
//
// The function does not implement full YAML parsing; it is designed for the
// structure emitted by Speakeasy in gen.lock.
func parseGeneratedFiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		inList           bool
		files            []string
		listIndentSpaces int
	)

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Normalize line endings and ignore comments that fully occupy a line
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// allow blank lines
			continue
		}

		// Detect start of generatedFiles list
		if !inList {
			// Accept lines like "generatedFiles:" with optional leading spaces
			if !strings.HasPrefix(trimmed, "generatedFiles:") {
				continue
			}
			inList = true
			// Indentation is the number of leading spaces before 'g'
			listIndentSpaces = countLeadingSpaces(line)
			continue
		}

		// If we're in the list, capture items beginning with '-' at a deeper indent
		// than the list key. Break when indentation decreases to list level or a
		// new top-level key appears.
		indent := countLeadingSpaces(line)

		// If indentation is less than or equal to the list key, we likely exited the list
		if indent <= listIndentSpaces {
			// End of list
			break
		}

		// Expect a dash item possibly after indentation: "- value"
		// Find first non-space index
		i := firstNonSpaceIndex(line)
		if i == -1 {
			// shouldn't happen since trimmed != ""
			continue
		}
		if line[i] != '-' {
			// Non-item encountered at greater indent
			if len(files) > 0 {
				break // End of the block
			}
			continue
		}

		// After '-' may be a space, then the value
		val := strings.TrimSpace(line[i+1:])
		if val == "" {
			// Multiline or complex YAML not supported
			return nil, fmt.Errorf("unsupported YAML structure for generatedFiles at line %d: empty item", lineNum)
		}

		// Strip surrounding quotes if present
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			if len(val) < 2 {
				continue
			}
			val = val[1 : len(val)-1]
		}

		files = append(files, val)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, errors.New("no generatedFiles entries found in gen.lock")
	}

	return files, nil
}

func countLeadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' && r != '\t' {
			return n
		}
		if r == ' ' {
			n++
			continue
		}
		// treat tab as +2 spaces (arbitrary but consistent)
		n += 2
	}
	return n
}

func firstNonSpaceIndex(s string) int {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return -1
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

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
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

// cleanupEmptyDirectories removes empty directories after file deletion
func cleanupEmptyDirectories(baseDir string, dirsToCheck map[string]bool, verbose bool, logger log.Logger) int {
	removed := 0
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return 0
	}

	// Keep trying to remove empty directories until no more can be removed
	// (handles nested empty directories)
	for {
		removedThisPass := 0
		for dir := range dirsToCheck {
			// Don't try to remove the base directory itself
			absDir, err := filepath.Abs(dir)
			if err != nil || absDir == absBaseDir {
				continue
			}

			// Check if directory is empty
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			if len(entries) == 0 {
				if err := os.Remove(dir); err == nil {
					removedThisPass++
					if verbose {
						relDir, _ := filepath.Rel(baseDir, dir)
						logger.Infof("  Removed empty directory: %s", relDir)
					}
					// Add parent directory to check
					parentDir := filepath.Dir(dir)
					dirsToCheck[parentDir] = true
				}
				delete(dirsToCheck, dir)
			}
		}

		removed += removedThisPass
		if removedThisPass == 0 {
			break
		}
	}

	return removed
}
