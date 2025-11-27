package integration_tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// customCodeTestOptions configures the test directory setup
type customCodeTestOptions struct {
	persistentEditsEnabled string // "", "true", or "never"
}

// setupCustomCodeTest creates a test directory for custom code prompt tests.
// It creates spec, workflow, gen.yaml, runs initial generation, initializes git,
// deletes sdk.go and commits - simulating user deletion of a generated file.
func setupCustomCodeTest(t *testing.T, opts customCodeTestOptions) string {
	t.Helper()

	temp := setupTestDir(t)

	// Create a minimal OpenAPI spec
	specContent := `openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
`
	err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0755)
	require.NoError(t, err)

	// Create workflow.yaml
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"test-source": {
				Inputs: []workflow.Document{
					{Location: "spec.yaml"},
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
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Create gen.yaml with optional persistentEdits config
	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true`

	if opts.persistentEditsEnabled != "" {
		genYamlContent += `
  persistentEdits:
    enabled: ` + opts.persistentEditsEnabled
	}

	genYamlContent += `
go:
  version: 1.0.0
  packageName: testsdk
`
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Create .genignore
	err = os.WriteFile(filepath.Join(temp, ".genignore"), []byte("go.mod\ngo.sum\n"), 0644)
	require.NoError(t, err)

	// Initial generation using speakeasy run
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")

	// Initialize git and commit
	runGit(t, temp, "init")
	runGit(t, temp, "config", "user.email", "test@example.com")
	runGit(t, temp, "config", "user.name", "Test User")
	runGit(t, temp, "add", "-A")
	runGit(t, temp, "commit", "-m", "initial generation")

	// Verify sdk.go exists, then delete it to simulate user deletion
	sdkFile := filepath.Join(temp, "sdk.go")
	require.FileExists(t, sdkFile)
	err = os.Remove(sdkFile)
	require.NoError(t, err)

	// Commit the deletion
	runGit(t, temp, "add", "-A")
	runGit(t, temp, "commit", "-m", "user deleted sdk.go")

	return temp
}

// runGit is a helper to run git commands
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(output))
}

// runGenerateSDK runs the generate sdk command and returns combined output
func runGenerateSDK(t *testing.T, testDir string, extraArgs ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	mainGo := filepath.Join(baseFolder, "main.go")

	args := []string{"run", mainGo, "generate", "sdk",
		"--lang", "go",
		"--schema", filepath.Join(testDir, "spec.yaml"),
		"--out", testDir,
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = testDir
	cmd.Env = append(os.Environ(),
		"SPEAKEASY_API_KEY=test-key",
		"SPEAKEASY_SERVER_URL=https://api.speakeasyapi.dev",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	t.Logf("Command output:\n%s", outputStr)

	if ctx.Err() == context.DeadlineExceeded {
		return outputStr, context.DeadlineExceeded
	}

	return outputStr, err
}

// TestCustomCodePrompt_SkipsPromptWithAutoYes tests that --auto-yes skips the prompt
func TestCustomCodePrompt_SkipsPromptWithAutoYes(t *testing.T) {
	t.Parallel()

	testDir := setupCustomCodeTest(t, customCodeTestOptions{})
	output, err := runGenerateSDK(t, testDir, "--auto-yes")

	// Should complete without timeout
	require.NotEqual(t, context.DeadlineExceeded, err, "Command should not timeout with --auto-yes")

	// Should not show prompt text
	assert.NotContains(t, output, "Would you like to enable custom code", "Should not show prompt with --auto-yes")
}

// TestCustomCodePrompt_SkipsPromptWhenNever tests that persistentEdits.enabled=never skips the prompt
func TestCustomCodePrompt_SkipsPromptWhenNever(t *testing.T) {
	t.Parallel()

	testDir := setupCustomCodeTest(t, customCodeTestOptions{persistentEditsEnabled: "never"})
	output, err := runGenerateSDK(t, testDir)

	// Should complete without timeout
	require.NotEqual(t, context.DeadlineExceeded, err, "Command should not timeout with persistentEdits.enabled=never")

	// Should not show prompt text
	assert.NotContains(t, output, "Would you like to enable custom code", "Should not show prompt with persistentEdits.enabled=never")
}

// TestCustomCodePrompt_ShowsPromptWhenDirty tests that prompt IS shown when dirty files
// are detected with default config. We prove this by showing the process hangs waiting
// for user input (times out), while --auto-yes makes it complete immediately.
func TestCustomCodePrompt_ShowsPromptWhenDirty(t *testing.T) {
	t.Parallel()

	testDir := setupCustomCodeTest(t, customCodeTestOptions{})

	// Run without --auto-yes with a SHORT timeout - should hang waiting for prompt input
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	mainGo := filepath.Join(baseFolder, "main.go")

	cmd := exec.CommandContext(ctx, "go", "run", mainGo,
		"generate", "sdk",
		"--lang", "go",
		"--schema", filepath.Join(testDir, "spec.yaml"),
		"--out", testDir,
	)
	cmd.Dir = testDir
	cmd.Env = append(os.Environ(),
		"SPEAKEASY_API_KEY=test-key",
		"SPEAKEASY_SERVER_URL=https://api.speakeasyapi.dev",
	)

	output, _ := cmd.CombinedOutput()
	outputStr := string(output)
	t.Logf("Command output:\n%s", outputStr)

	// The command should timeout because it's waiting for user input on the prompt
	// This proves the prompt IS being shown
	require.Equal(t, context.DeadlineExceeded, ctx.Err(),
		"Command should timeout waiting for prompt input. If it completed, the prompt wasn't shown. Output: %s", outputStr)

	// Additionally verify the output contains prompt-related text before timeout
	assert.Contains(t, outputStr, "Changes detected", "Output should contain prompt text before hanging")
}
