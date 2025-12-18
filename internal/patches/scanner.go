package patches

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
)

// Scanner scans a directory for files containing @generated-id UUIDs.
// This enables tracking file identity across renames/moves.
type Scanner struct {
	rootDir string
}

// NewScanner creates a new Scanner for the given root directory.
func NewScanner(rootDir string) *Scanner {
	return &Scanner{rootDir: rootDir}
}

// generatedIDPattern matches @generated-id: <ID> in file headers.
// Short IDs are 12 hex chars (e.g., a1b2c3d4e5f6)
var generatedIDPattern = regexp.MustCompile(`@generated-id:\s*([a-f0-9]{12})`)

// ScanResult contains the mapping of UUIDs to file paths.
type ScanResult struct {
	// UUIDToPath maps file UUIDs to their current relative paths.
	// Used to detect file moves/renames.
	UUIDToPath map[string]string

	// PathToUUID maps relative paths to their UUIDs.
	PathToUUID map[string]string
}

// Scan walks the directory tree and finds all files with @generated-id headers.
// It returns a bidirectional mapping between UUIDs and file paths.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		UUIDToPath: make(map[string]string),
		PathToUUID: make(map[string]string),
	}

	err := filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if d.IsDir() {
			// Skip common non-generated directories
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".venv" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary files by checking content for null bytes
		if isBinaryFile(path) {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return nil
		}

		// Normalize to forward slashes (git/lockfile convention)
		relPath = filepath.ToSlash(relPath)

		// Try to extract ID from file header (supports both UUID and short ID formats)
		id, err := extractGeneratedIDFromFile(path)
		if err != nil || id == "" {
			return nil
		}

		result.UUIDToPath[id] = relPath
		result.PathToUUID[relPath] = id

		return nil
	})

	return result, err
}

// extractGeneratedIDFromFile reads the first few lines of a file looking for @generated-id.
// Returns the ID (either legacy UUID or short 12-char ID format).
func extractGeneratedIDFromFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Only check first 20 lines - the ID should be near the top
	for i := 0; i < 20 && scanner.Scan(); i++ {
		line := scanner.Text()
		if match := generatedIDPattern.FindStringSubmatch(line); len(match) > 1 {
			return match[1], nil
		}
	}

	return "", nil
}

// isBinaryFile checks if a file is binary by reading first 8KB and checking for null bytes.
func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 8192)
	n, _ := file.Read(buf)
	return generate.IsBinaryContent(buf[:n])
}
