package integration_tests

import (
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

const (
	tempDir      = "temp"
	letterBytes  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func createTempDir() (string, error) {
	temp := fmt.Sprintf("%s/%s", tempDir, randStringBytes(7))
	if err := os.Mkdir(temp, 0o755); err != nil {
		return "", err
	}

	return temp, nil
}

func isLocalFileReference(filePath string) bool {
	u, err := url.Parse(filePath)
	if err != nil {
		return true
	}

	return u.Scheme == "" || u.Scheme == "file"
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

var randStringBytes = func(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func expectedFilesByLanguage(language string) []string {
	switch language {
	case "go":
		return []string{"README.md", "sdk.go", "go.mod"}
	case "typescript":
		return []string{"README.md", "package.json", "tsconfig.json"}
	case "python":
		return []string{"README.md", "setup.py"}
	default:
		return []string{}
	}
}

func checkForExpectedFiles(t *testing.T, outdir string, files []string) {
	for _, fileName := range files {
		filePath := filepath.Join(outdir, fileName)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("Error checking file %s in directory %s: %v", fileName, outdir, err)
			continue
		}

		if fileInfo.Size() == 0 {
			t.Errorf("Expected file %s in directory %s is empty.", fileName, outdir)
		}
	}
}
