package integration_tests

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	tempDir      = "temp"
	letterBytes  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	version      = "0.0.1"
	artifactArch = "linux_amd64"
)

func createTempDir(wd string) (string, error) {
	target := filepath.Join(wd, tempDir, randStringBytes(7))
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", err
	}

	return target, nil
}

func isLocalFileReference(filePath string) bool {
	u, err := url.Parse(filePath)
	if err != nil {
		return true
	}

	return u.Scheme == "" || u.Scheme == "file"
}

func copyFile(src, dst string) error {
	_, filename, _, _ := runtime.Caller(0)
	targetSrc := filepath.Join(filepath.Dir(filename), src)

	sourceFile, err := os.Open(targetSrc)
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
	case "mcp-typescript", "typescript":
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

type Runnable interface {
	Run() error
}

type subprocessRunner struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

func (r *subprocessRunner) Run() error {
	err := r.cmd.Run()
	if err != nil {
		fmt.Println(r.out.String())
		return err
	}
	return nil
}

func execute(t *testing.T, wd string, args ...string) Runnable {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	
	// Build the CLI binary first
	binaryPath := filepath.Join(os.TempDir(), "speakeasy-test-"+randStringBytes(8))
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = baseFolder
	buildCmd.Env = os.Environ()
	
	buildOut := bytes.Buffer{}
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI binary: %v\nOutput: %s", err, buildOut.String())
	}
	
	// Clean up binary after test
	t.Cleanup(func() {
		os.Remove(binaryPath)
	})
	
	// Execute the built binary from the test directory
	execCmd := exec.Command(binaryPath, args...)
	execCmd.Env = os.Environ()
	execCmd.Dir = wd

	// store stdout and stderr in a buffer and output it all in one go if there's a failure
	out := bytes.Buffer{}
	execCmd.Stdout = &out
	execCmd.Stderr = &out

	return &subprocessRunner{
		cmd: execCmd,
		out: &out,
	}
}
