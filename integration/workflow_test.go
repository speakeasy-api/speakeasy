package integration_tests

import (
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/assert"
)

const (
	tempDir     = "temp"
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func TestMain(m *testing.M) {
	// Create a temporary directory
	if _, err := os.Stat(tempDir); err == nil {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}

	if err := os.Mkdir(tempDir, 0o755); err != nil {
		panic(err)
	}

	// Defer the removal of the temp directory
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}()

	code := m.Run()
	os.Exit(code)
}

func TestCodeSampleWorkflows(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		outdir     string
		inputDoc   string
		withForce  bool
	}{
		{
			name:       "codeSamples with remote document",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml",
		},
		{
			name:       "codeSamples with local document",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "spec.yaml",
		},
		{
			name:       "codeSamples with json output",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.json",
		},
		{
			name:       "codeSamples with force generate",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "spec.yaml",
			withForce:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp, err := createTempDir()
			assert.NoError(t, err)

			workflowFile := &workflow.Workflow{
				Version: workflow.WorkflowVersion,
				Sources: make(map[string]workflow.Source),
				Targets: make(map[string]workflow.Target),
			}
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs: []workflow.Document{
					{
						Location: tt.inputDoc,
					},
				},
			}
			workflowFile.Targets["first-target"] = workflow.Target{
				Target: tt.targetType,
				Source: "first-source",
				Output: &tt.outdir,
				CodeSamples: &workflow.CodeSamples{
					Output: "codeSamples.yaml",
				},
			}

			if isLocalFileReference(tt.inputDoc) {
				err := copyFile("resources/spec.yaml", fmt.Sprintf("%s/%s", temp, tt.inputDoc))
				assert.NoError(t, err)
			}

			err = workflowFile.Validate(generate.GetSupportedLanguages())
			assert.NoError(t, err)
			err = os.MkdirAll(fmt.Sprintf("%s/.speakeasy", temp), 0o755)
			assert.NoError(t, err)
			workflow.Save("temp", workflowFile)
			assert.NoError(t, err)
			// cleanupTempDir(temp)
		})
	}
}

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

func cleanupTempDir(temp string) error {
	return os.RemoveAll(temp)
}

var randStringBytes = func(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
