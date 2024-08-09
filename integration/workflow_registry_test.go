package integration_tests

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestFrozenWorkflowLock(t *testing.T) {
	t.Parallel()
	temp := setupTestDir(t)

	// Create a basic workflow file
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"test-source": {
				Inputs: []workflow.Document{
					{Location: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml"},
				},
			},
		},
		Targets: map[string]workflow.Target{
			"test-target": {
				Target: "go",
				Source: "test-source",
			},
		},
	}

	err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Run the initial generation
	initialArgs := []string{"run", "-t", "all"}
	cmdErr := executeI(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	// Calculate checksums of generated files
	initialChecksums, err := calculateChecksums(temp)
	require.NoError(t, err)

	// Modify the source OpenAPI spec to simulate a change
	// Shouldn't do anything; we'll validate that later.
	err = modifyOpenAPISpec(temp)
	require.NoError(t, err)

	// Run with --frozen-workflow-lock
	frozenArgs := []string{"run", "-t", "all", "--frozen-workflow-lock"}
	cmdErr = executeI(t, temp, frozenArgs...).Run()
	require.NoError(t, cmdErr)

	// Calculate checksums after frozen run
	frozenChecksums, err := calculateChecksums(temp)
	require.NoError(t, err)

	// Compare checksums
	require.Equal(t, initialChecksums, frozenChecksums, "Generated files should be identical when using --frozen-workflow-lock")
}

func calculateChecksums(dir string) (map[string]string, error) {
	checksums := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relPath, _ := filepath.Rel(dir, path)
			checksums[relPath] = fmt.Sprintf("%x", md5.Sum(data))
		}
		return nil
	})
	return checksums, err
}

func modifyOpenAPISpec(dir string) error {
	specPath := filepath.Join(dir, ".speakeasy", "workflow.lock")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}

	// Modify the spec content (this is a simplistic change, you might want to do something more sophisticated)
	modifiedData := strings.Replace(string(data), "Swagger Petstore", "Modified Petstore", 1)

	return os.WriteFile(specPath, []byte(modifiedData), 0644)
}