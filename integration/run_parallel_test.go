package integration_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

// TestRunParallelSpecificTargets verifies that `speakeasy run --parallel -t a,b`
// runs only the requested subset of targets in parallel, leaving other targets
// in the workflow untouched.
func TestRunParallelSpecificTargets(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	temp := t.TempDir()

	require.NoError(t, copyFile("resources/multi_root.yaml", filepath.Join(temp, "multi_root.yaml")))
	require.NoError(t, copyFile("resources/multi_components.yaml", filepath.Join(temp, "multi_components.yaml")))

	// Three typescript targets, each writing to its own output directory.
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"src": {
				Inputs: []workflow.Document{
					{Location: workflow.LocationString("multi_root.yaml")},
				},
			},
		},
		Targets: map[string]workflow.Target{
			"ts-a": {Target: "typescript", Source: "src", Output: stringPtr("ts-a")},
			"ts-b": {Target: "typescript", Source: "src", Output: stringPtr("ts-b")},
			"ts-c": {Target: "typescript", Source: "src", Output: stringPtr("ts-c")},
		},
	}
	require.NoError(t, os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755))
	require.NoError(t, workflow.Save(temp, workflowFile))

	genYaml := func(pkg string) string {
		return `configVersion: 2.0.0
generation:
  sdkClassName: SDK
typescript:
  version: 0.0.1
  packageName: ` + pkg + `
`
	}
	for dir, pkg := range map[string]string{"ts-a": "pkga", "ts-b": "pkgb", "ts-c": "pkgc"} {
		require.NoError(t, os.MkdirAll(filepath.Join(temp, dir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(temp, dir, "gen.yaml"), []byte(genYaml(pkg)), 0o644))
	}

	gitInit(t, temp)

	// Run only the ts-a and ts-b subset in parallel; ts-c is intentionally excluded.
	args := []string{"run", "--parallel", "-t", "ts-a,ts-b", "--pinned", "--force", "--skip-versioning", "--skip-compile"}
	require.NoError(t, execute(t, temp, args...).Run())

	// The two requested targets were generated.
	checkForExpectedFiles(t, filepath.Join(temp, "ts-a"), expectedFilesByLanguage("typescript"))
	checkForExpectedFiles(t, filepath.Join(temp, "ts-b"), expectedFilesByLanguage("typescript"))

	// The excluded target was not generated (only the gen.yaml we wrote is present).
	require.NoFileExists(t, filepath.Join(temp, "ts-c", "package.json"))
	require.NoFileExists(t, filepath.Join(temp, "ts-c", "README.md"))
}

// TestRunParallelUnknownTarget verifies that requesting a target that does not
// exist in the workflow fails fast with a clear error.
func TestRunParallelUnknownTarget(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	temp := t.TempDir()

	require.NoError(t, copyFile("resources/multi_root.yaml", filepath.Join(temp, "multi_root.yaml")))
	require.NoError(t, copyFile("resources/multi_components.yaml", filepath.Join(temp, "multi_components.yaml")))

	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"src": {
				Inputs: []workflow.Document{
					{Location: workflow.LocationString("multi_root.yaml")},
				},
			},
		},
		Targets: map[string]workflow.Target{
			"ts-a": {Target: "typescript", Source: "src", Output: stringPtr("ts-a")},
			"ts-b": {Target: "typescript", Source: "src", Output: stringPtr("ts-b")},
		},
	}
	require.NoError(t, os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755))
	require.NoError(t, workflow.Save(temp, workflowFile))

	gitInit(t, temp)

	args := []string{"run", "--parallel", "-t", "ts-a,ts-nope", "--pinned", "--skip-compile"}
	require.Error(t, execute(t, temp, args...).Run())
}
