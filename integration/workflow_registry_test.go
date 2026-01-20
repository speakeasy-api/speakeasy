package integration_tests

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

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
	err = os.WriteFile(filepath.Join(temp, ".speakeasy", "gen.yaml"), []byte(genYamlContent), 0o644)
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

	// Find differences to help debug test failures
	tempDir := os.TempDir()
	for key, val := range initialChecksums {
		if rerunVal, ok := rerunChecksums[key]; ok {
			if val != rerunVal {
				t.Logf("File differs: %s", key)
				// Save files for comparison
				initialPath := filepath.Join(tempDir, "initial_"+filepath.Base(key))
				rerunPath := filepath.Join(tempDir, "rerun_"+filepath.Base(key))
				_ = os.WriteFile(initialPath, []byte(val), 0o644)
				_ = os.WriteFile(rerunPath, []byte(rerunVal), 0o644)
				t.Logf("Saved to %s and %s", initialPath, rerunPath)
				t.Logf("Initial (first 200): %s", truncate(val, 200))
				t.Logf("Rerun (first 200): %s", truncate(rerunVal, 200))
			}
		} else {
			t.Logf("File missing in rerun: %s", key)
		}
	}
	for key := range rerunChecksums {
		if _, ok := initialChecksums[key]; !ok {
			t.Logf("New file in rerun: %s", key)
		}
	}

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
	require.NotEmpty(t, registryLocation, "registry location should be set")

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
	require.NotEmpty(t, registryLocation, "registry location should be set")

	print(registryLocation)

	workflowFile.Sources["test-source"].Inputs[0].Location = workflow.LocationString(registryLocation)
	require.NoError(t, workflow.Save(temp, workflowFile))

	// Re-run the generation. It should work.
	cmdErr = execute(t, temp, initialArgs...).Run()
	require.NoError(t, cmdErr)
}

// TestFrozenWorkflowLockWithRegistryInput verifies --frozen-workflow-lockfile works with registry URLs.
func TestFrozenWorkflowLockWithRegistryInput(t *testing.T) {
	t.Parallel()

	temp := setupTestDir(t)

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

	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
typescript:
  version: 0.0.1
  packageName: openapi
`
	require.NoError(t, os.WriteFile(filepath.Join(temp, ".speakeasy", "gen.yaml"), []byte(genYamlContent), 0o644))

	// Initial generation publishes to registry
	initialArgs := []string{"run", "-t", "all", "--force", "--pinned", "--skip-versioning", "--skip-compile"}
	require.NoError(t, execute(t, temp, initialArgs...).Run())

	// Switch input to registry URL
	workflowFile, _, err = workflow.Load(temp)
	require.NoError(t, err)
	registryLocation := workflowFile.Sources["test-source"].Registry.Location.String()
	require.NotEmpty(t, registryLocation, "registry location should be set")
	workflowFile.Sources["test-source"].Inputs[0].Location = workflow.LocationString(registryLocation)
	require.NoError(t, workflow.Save(temp, workflowFile))

	// Run with registry input to establish baseline
	require.NoError(t, execute(t, temp, initialArgs...).Run())
	initialChecksums, err := filesToString(temp)
	require.NoError(t, err)

	// Modify workflow in memory (don't save) to simulate a spec change
	workflowFile.Sources["test-source"].Inputs[0].Location = "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.1/petstore.yaml"

	// Run with frozen lock - should use lockfile revision, not the modified input
	frozenArgs := []string{"run", "-t", "all", "--pinned", "--frozen-workflow-lockfile", "--skip-compile"}
	require.NoError(t, execute(t, temp, frozenArgs...).Run())
	frozenChecksums, err := filesToString(temp)
	require.NoError(t, err)

	delete(frozenChecksums, ".speakeasy/gen.lock")
	delete(initialChecksums, ".speakeasy/gen.lock")
	require.Equal(t, initialChecksums, frozenChecksums, "Generated files should be identical when using --frozen-workflow-lockfile with registry input")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func filesToString(dir string) (map[string]string, error) {
	checksums := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip .git directory - git objects change on each run due to timestamps
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relPath, _ := filepath.Rel(dir, path)
			// Normalize path separators to forward slashes for cross-platform consistency
			relPath = filepath.ToSlash(relPath)
			checksums[relPath] = string(data)
		}
		return nil
	})
	return checksums, err
}
