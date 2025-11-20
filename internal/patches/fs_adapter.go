package patches

import (
	"os"
	"path/filepath"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/patches"
)

// FSAdapter implements the patches.FileSystem interface for disk I/O operations.
// It provides file read/write operations relative to a root directory.
type FSAdapter struct {
	rootDir string
}

var _ patches.FileSystem = (*FSAdapter)(nil)

// NewFSAdapter creates a new FSAdapter rooted at the given directory.
func NewFSAdapter(rootDir string) *FSAdapter {
	return &FSAdapter{rootDir: rootDir}
}

// ReadFile returns content from the user's disk.
// Returns os.ErrNotExist if the file is missing.
func (f *FSAdapter) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(f.rootDir, path)
	return os.ReadFile(fullPath)
}

// WriteFile writes content to disk.
// It creates parent directories if needed (mkdir -p behavior).
// If isExecutable is true, sets mode 0755; otherwise 0644.
func (f *FSAdapter) WriteFile(path string, content []byte, isExecutable bool) error {
	fullPath := filepath.Join(f.rootDir, path)

	// Ensure parent directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Determine file mode
	mode := os.FileMode(0644)
	if isExecutable {
		mode = 0755
	}

	return os.WriteFile(fullPath, content, mode)
}

// RemoveFile deletes a file from disk.
func (f *FSAdapter) RemoveFile(path string) error {
	fullPath := filepath.Join(f.rootDir, path)
	return os.Remove(fullPath)
}

// ScanForGeneratedIDs scans the root directory for files with @generated-id headers.
// Returns a map of UUID -> relative file path.
func (f *FSAdapter) ScanForGeneratedIDs() (map[string]string, error) {
	scanner := NewScanner(f.rootDir)
	result, err := scanner.Scan()
	if err != nil {
		return nil, err
	}
	return result.UUIDToPath, nil
}
