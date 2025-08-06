package integration_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestMultiFileStability(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	temp := setupTestDir(t)

	// Copy the multi-file OpenAPI spec files
	err := copyFile("resources/multi_root.yaml", filepath.Join(temp, "multi_root.yaml"))
	require.NoError(t, err)
	err = copyFile("resources/multi_components.yaml", filepath.Join(temp, "multi_components.yaml"))
	require.NoError(t, err)

	// Create a workflow file with multi-file input
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"multi-file-source": {
				Inputs: []workflow.Document{
					{Location: workflow.LocationString("multi_root.yaml")},
				},
			},
		},
		Targets: map[string]workflow.Target{
			"multi-file-target": {
				Target: "typescript",
				Source: "multi-file-source",
			},
		},
	}

	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Create gen.yaml with persistentEdits enabled for stability
	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  persistentEdits:
    enabled: "true"
typescript:
  version: 0.0.1
  packageName: openapi
`
	err = os.WriteFile(filepath.Join(temp, ".speakeasy", "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Initialize git repo (required for persistentEdits)
	gitInit(t, temp)

	// Run the initial generation
	var initialChecksums map[string]string
	initialArgs := []string{"run", "-t", "all", "--force", "--pinned", "--skip-versioning", "--skip-compile"}
	cmdErr := execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	// Calculate checksums of generated files
	initialChecksums, err = filesToString(temp)
	require.NoError(t, err)

	// Re-run the generation. We should have stable digests.
	cmdErr = execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
	rerunChecksums, err := filesToString(temp)
	require.NoError(t, err)

	// Compare checksums to ensure stability
	require.Equal(t, initialChecksums, rerunChecksums, "Generated files should be identical for multi-file OpenAPI specs")

	// Test frozen workflow lock behavior
	frozenArgs := []string{"run", "-t", "all", "--pinned", "--frozen-workflow-lockfile", "--skip-compile"}
	cmdErr = execute(t, temp, frozenArgs...).Run()
	require.NoError(t, cmdErr)

	// Calculate checksums after frozen run
	frozenChecksums, err := filesToString(temp)
	require.NoError(t, err)

	// exclude gen.lock -- we could (we do) reformat the document inside the frozen one
	delete(frozenChecksums, ".speakeasy/gen.lock")
	delete(initialChecksums, ".speakeasy/gen.lock")

	// Compare checksums
	if diff := cmp.Diff(initialChecksums, frozenChecksums); diff != "" {
		t.Fatalf("Generated files should be identical when using --frozen-workflow-lock with multi-file specs. Mismatch (-want +got):\n%s", diff)
	}
}
