package integration_tests

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestStability(t *testing.T) {
	temp := setupTestDir(t)

	// Create a basic workflow file
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"test-source": {
				Inputs: []workflow.Document{
					{Location: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.yaml"},
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
	initialChecksums, err = filesToString(temp)
	require.NoError(t, err)

	// Re-run the generation. We should have stable digests.
	cmdErr = execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
	rerunChecksums, err := filesToString(temp)
	require.NoError(t, err)
	require.Equal(t, initialChecksums, rerunChecksums, "Generated files should be identical")
	// Modify the workflow file to simulate a change
	// Shouldn't do anything; we'll validate that later.
	workflowFile.Sources["test-source"].Inputs[0].Location = "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.1/petstore.yaml"
	require.NoError(t, err)

	// Run with --frozen-workflow-lock
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
	require.Equal(t, initialChecksums, frozenChecksums, "Generated files should be identical when using --frozen-workflow-lock")
}

func TestRegistryFlow(t *testing.T) {
	t.Parallel()
	temp := setupTestDir(t)

	// Create a basic workflow file
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"test-source": {
				Inputs: []workflow.Document{
					{Location: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.yaml"},
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
	require.NoError(t, workflow.Save(temp, workflowFile))

	// Run the initial generation
	initialArgs := []string{"run", "-t", "all", "--force", "--pinned", "--skip-versioning", "--skip-compile"}
	cmdErr := execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	// Get the registry location and set it to the source input
	workflowFile, _, err = workflow.Load(temp)
	require.NoError(t, err)
	registryLocation := workflowFile.Sources["test-source"].Registry.Location.String()
	require.True(t, len(registryLocation) > 0, "registry location should be set")

	workflowFile.Sources["test-source"].Inputs[0].Location = workflow.LocationString(registryLocation)
	require.NoError(t, workflow.Save(temp, workflowFile))

	// Re-run the generation. It should work.
	cmdErr = execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
}

func TestRegistryFlow_JSON(t *testing.T) {
	t.Parallel()
	temp := setupTestDir(t)

	// Create a basic workflow file
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"test-source": {
				Inputs: []workflow.Document{
					{Location: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.json"},
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
	b, err := yaml.Marshal(workflowFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(temp, ".speakeasy", "workflow.yaml"), b, 0o644))

	// Run the initial generation
	initialArgs := []string{"run", "-t", "all", "--force", "--pinned", "--skip-versioning", "--skip-compile"}
	cmdErr := execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)

	// Get the registry location and set it to the source input
	workflowFile, _, err = workflow.Load(temp)
	require.NoError(t, err)
	registryLocation := workflowFile.Sources["test-source"].Registry.Location.String()
	require.True(t, len(registryLocation) > 0, "registry location should be set")

	print(registryLocation)

	workflowFile.Sources["test-source"].Inputs[0].Location = workflow.LocationString(registryLocation)
	require.NoError(t, workflow.Save(temp, workflowFile))

	// Re-run the generation. It should work.
	cmdErr = execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
}

func filesToString(dir string) (map[string]string, error) {
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
			checksums[relPath] = string(data)
		}
		return nil
	})
	return checksums, err
}
