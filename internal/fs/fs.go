package fs

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/filesystem"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
)

// FileSystem is a wrapper around the os.FS type that implements the filesystem.FileSystem interface needed by the openapi-generation package
type FileSystem struct {
	outDir string
}

var _ filesystem.FileSystem = &FileSystem{}

// NewFileSystem creates a new FileSystem.
func NewFileSystem(outDir string) *FileSystem {
	return &FileSystem{outDir}
}

func (f *FileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *FileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (f *FileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *FileSystem) Open(path string) (fs.File, error) {
	return os.Open(path)
}

func (f *FileSystem) OpenFile(path string, flag int, perm os.FileMode) (filesystem.File, error) {
	return os.OpenFile(path, flag, perm)
}

func (f *FileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (f *FileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (f *FileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// generatedIDPattern matches @generated-id: <ID> in file headers.
// Short IDs are 12 hex chars (e.g., a1b2c3d4e5f6)
var generatedIDPattern = regexp.MustCompile(`@generated-id:\s*([a-f0-9]{12})`)

// ScanForGeneratedIDs scans the root directory for files with @generated-id headers.
// Returns a map of ID -> relative file path.
// This is used to detect when users have moved generated files.
func (f *FileSystem) ScanForGeneratedIDs() (map[string]string, error) {
	if f.outDir == "" {
		return nil, nil
	}

	result := make(map[string]string)

	err := filepath.WalkDir(f.outDir, func(path string, d fs.DirEntry, err error) error {
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
		relPath, err := filepath.Rel(f.outDir, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)

		// Try to extract ID from file header
		id, err := extractGeneratedIDFromFile(path)
		if err != nil || id == "" {
			return nil
		}

		result[id] = relPath

		return nil
	})

	return result, err
}

// extractGeneratedIDFromFile reads the first few lines of a file looking for @generated-id.
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
