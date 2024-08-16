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

func TestStability(t *testing.T) {
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
				Target: "typescript",
				Source: "test-source",
			},
		},
	}

	err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Run the initial generation
	var initialChecksums map[string]string
	initialArgs := []string{"run", "-t", "all", "--force", "--pinned", "--skip-versioning", "--skip-compile"}
	cmdErr := executeI(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	// Calculate checksums of generated files
	initialChecksums, err = calculateChecksums(temp)
	require.NoError(t, err)

	// Re-run the generation. We should have stable digests.
	cmdErr = executeI(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	rerunChecksums, err := calculateChecksums(temp)
	require.NoError(t, err)
	require.Equal(t, initialChecksums, rerunChecksums, "Generated files should be identical when using --frozen-workflow-lock")
	// Once more, just to be sure. This now has a change report so it could in theory change.
	cmdErr = executeI(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
	rerunChecksums, err = calculateChecksums(temp)
	require.NoError(t, err)
	require.Equal(t, initialChecksums, rerunChecksums, "Generated files should be identical when using --frozen-workflow-lock")
	// Modify the workflow file to simulate a change
	// Shouldn't do anything; we'll validate that later.
	workflowFile.Sources["test-source"].Inputs[0].Location = "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.1/petstore.yaml"
	require.NoError(t, err)

	// Run with --frozen-workflow-lock
	frozenArgs := []string{"run", "-t", "all", "--pinned", "--frozen-workflow-lockfile", "--skip-compile"}
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
		// Skip .git
		if strings.Contains(path, ".git") {
			return nil
		}
		// skip gen.lock. In particular, this currently varies between runs if the OAS is reformatted which occurs during bundling
		// however we validate stability through the workflow lock file
		// TODO: once https://github.com/pb33f/libopenapi/issues/321 is closed, use this to power document checksums and drop this if
		if strings.Contains(path, "gen.lock") {
			return nil
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