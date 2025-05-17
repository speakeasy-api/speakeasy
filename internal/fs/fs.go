package fs

import (
	"io/fs"
	"os"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/filesystem"
)

// FileSystem is a wrapper around the os.FS type that implements the filesystem.FileSystem interface needed by the openapi-generation package
type FileSystem struct {
}

var _ filesystem.FileSystem = &FileSystem{}

func NewFileSystem() *FileSystem {
	return &FileSystem{}
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
