package integration_tests

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestCustomCodeWorkflows(t *testing.T) {
	t.Parallel()

	// Build the speakeasy binary once for all tests
	speakeasyBinary := buildSpeakeasyBinary(t)

	tests := []struct {
		name            string
		targetTypes     []string
		inputDoc        string
		withCodeSamples bool
	}{
		{
			name: "generation with local document",
			targetTypes: []string{
				"go",
			},
			inputDoc: "customcodespec.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Use custom-code-test directory to avoid gitignore issues from speakeasy repo
			temp := setupCustomCodeTestDir(t)

			// Create workflow file and associated resources
			workflowFile := &workflow.Workflow{
				Version: workflow.WorkflowVersion,
				Sources: make(map[string]workflow.Source),
				Targets: make(map[string]workflow.Target),
			}
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs: []workflow.Document{
					{
						Location: workflow.LocationString(tt.inputDoc),
					},
				},
			}

			for i := range tt.targetTypes {
				outdir := "go"
				target := workflow.Target{
					Target: tt.targetTypes[i],
					Source: "first-source",
					Output: &outdir,
				}
				if tt.withCodeSamples {
					target.CodeSamples = &workflow.CodeSamples{
						Output: "codeSamples.yaml",
					}
				}
				workflowFile.Targets[fmt.Sprintf("%d-target", i)] = target
			}

			if isLocalFileReference(tt.inputDoc) {
				err := copyFile("resources/customcodespec.yaml", fmt.Sprintf("%s/%s", temp, tt.inputDoc))
				require.NoError(t, err)
			}

			err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
			require.NoError(t, err)
			err = workflow.Save(temp, workflowFile)
			require.NoError(t, err)

			// Run speakeasy run command using the built binary
			runCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
			runCmd.Dir = temp
			runOutput, runErr := runCmd.CombinedOutput()
			require.NoError(t, runErr, "speakeasy run should succeed: %s", string(runOutput))

			// SDK directory where files are generated
			sdkDir := filepath.Join(temp, "go")

			// Run go mod tidy to ensure go.sum is properly populated
			// This is necessary because we used --skip-compile above
			goModTidyCmd := exec.Command("go", "mod", "tidy")
			goModTidyCmd.Dir = sdkDir
			output, err := goModTidyCmd.CombinedOutput()
			require.NoError(t, err, "Failed to run go mod tidy: %s", string(output))

			if tt.withCodeSamples {
				codeSamplesPath := filepath.Join(sdkDir, "codeSamples.yaml")
				content, err := os.ReadFile(codeSamplesPath)
				require.NoError(t, err, "No readable file %s exists", codeSamplesPath)

				// Check if codeSamples file is not empty and contains expected content
				require.NotEmpty(t, content, "codeSamples.yaml should not be empty")
			}

			// SDK is generated in go subdirectory
			for _, targetType := range tt.targetTypes {
				checkForExpectedFiles(t, sdkDir, expectedFilesByLanguage(targetType))
			}

			// Initialize git repository in the go directory
			initGitRepo(t, sdkDir)

			// Copy workflow.yaml and spec to SDK directory before committing
			copyWorkflowToSDK(t, temp, sdkDir)

			// Commit all generated files with "clean generation" message
			gitCommit(t, sdkDir, "clean generation")

			// Verify the commit was created with the correct message
			verifyGitCommit(t, sdkDir, "clean generation")

			// Modify httpmetadata.go to add custom code
			httpMetadataPath := filepath.Join(sdkDir, "models", "components", "httpmetadata.go")
			modifyLineInFile(t, httpMetadataPath, 10, "\t// custom code")

			// Run customcode command from SDK directory using the built binary
			customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
			customCodeCmd.Dir = sdkDir
			customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
			require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

			// Verify patches directory was created in SDK directory
			patchesDir := filepath.Join(sdkDir, ".speakeasy", "patches")
			_, err = os.Stat(patchesDir)
			require.NoError(t, err, "patches directory should exist at %s", patchesDir)

			// Verify patch file was created
			patchFile := filepath.Join(patchesDir, "custom-code.diff")
			_, err = os.Stat(patchFile)
			require.NoError(t, err, "patch file should exist at %s", patchFile)

			// Run speakeasy run again from the SDK directory to regenerate and apply patches
			regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
			regenCmd.Dir = sdkDir
			regenOutput, regenErr := regenCmd.CombinedOutput()
			require.NoError(t, regenErr, "speakeasy run should succeed on regeneration: %s", string(regenOutput))

			// Verify the custom code is still present after regeneration
			httpMetadataContent, err := os.ReadFile(httpMetadataPath)
			require.NoError(t, err, "Failed to read httpmetadata.go after regeneration")
			require.Contains(t, string(httpMetadataContent), "// custom code", "Custom code comment should still be present after regeneration")
		})
	}
}

// initGitRepo initializes a git repository in the specified directory
func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to initialize git repo: %s", string(output))

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to configure git user.email: %s", string(output))

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to configure git user.name: %s", string(output))
}

// gitCommit creates a git commit with all changes in the specified directory
func gitCommit(t *testing.T, dir, message string) {
	t.Helper()

	// Add all files
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to git add: %s", string(output))

	// Commit with message
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to git commit: %s", string(output))
}

// verifyGitCommit verifies that a git commit exists with the expected message
func verifyGitCommit(t *testing.T, dir, expectedMessage string) {
	t.Helper()

	// Check that .git directory exists
	gitDir := filepath.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	require.NoError(t, err, ".git directory should exist")

	// Get the latest commit message
	cmd := exec.Command("git", "log", "-1", "--pretty=%B")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to get git log: %s", string(output))

	// Verify commit message matches
	commitMessage := strings.TrimSpace(string(output))
	require.Equal(t, expectedMessage, commitMessage, "Commit message should match expected message")
}

// modifyLineInFile modifies a specific line in a file (1-indexed line number)
func modifyLineInFile(t *testing.T, filePath string, lineNumber int, newContent string) {
	t.Helper()

	// Read the file
	file, err := os.Open(filePath)
	require.NoError(t, err, "Failed to open file: %s", filePath)
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.NoError(t, scanner.Err(), "Failed to read file: %s", filePath)

	// Modify the specific line (convert 1-indexed to 0-indexed)
	require.Less(t, lineNumber, len(lines)+1, "Line number %d is out of range (file has %d lines)", lineNumber, len(lines))
	lines[lineNumber-1] = newContent

	// Write back to the file
	file, err = os.Create(filePath)
	require.NoError(t, err, "Failed to open file for writing: %s", filePath)
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		require.NoError(t, err, "Failed to write line to file")
	}
	require.NoError(t, writer.Flush(), "Failed to flush writer")
}

// buildSpeakeasyBinary builds the speakeasy binary and returns the path to it
func buildSpeakeasyBinary(t *testing.T) string {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	binaryPath := filepath.Join(baseFolder, "speakeasy-test-binary")

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, "./main.go")
	cmd.Dir = baseFolder
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build speakeasy binary: %s", string(output))

	// Clean up the binary when test completes
	t.Cleanup(func() {
		os.Remove(binaryPath)
	})

	return binaryPath
}

// copyWorkflowToSDK copies the workflow.yaml and spec files from the workspace to the SDK directory
func copyWorkflowToSDK(t *testing.T, workspaceDir, sdkDir string) {
	t.Helper()

	// Copy workflow.yaml file, removing output paths since we're running from SDK directory
	srcWorkflowPath := filepath.Join(workspaceDir, ".speakeasy", "workflow.yaml")
	dstWorkflowPath := filepath.Join(sdkDir, ".speakeasy", "workflow.yaml")
	workflowContent, err := os.ReadFile(srcWorkflowPath)
	require.NoError(t, err, "Failed to read workflow.yaml")

	// Remove "output: go" lines from the workflow content
	workflowStr := string(workflowContent)
	workflowStr = strings.ReplaceAll(workflowStr, "\n        output: go", "")

	err = os.WriteFile(dstWorkflowPath, []byte(workflowStr), 0o644)
	require.NoError(t, err, "Failed to write workflow.yaml")

	// Read the workflow to find spec files to copy
	workflowFile, _, err := workflow.Load(workspaceDir)
	require.NoError(t, err, "Failed to load workflow.yaml")

	// Copy local spec files to SDK directory
	for _, source := range workflowFile.Sources {
		for i := range source.Inputs {
			if isLocalFileReference(string(source.Inputs[i].Location)) {
				specPath := string(source.Inputs[i].Location)

				// Copy the spec file to SDK directory
				srcSpecPath := filepath.Join(workspaceDir, specPath)
				dstSpecPath := filepath.Join(sdkDir, specPath)

				specContent, err := os.ReadFile(srcSpecPath)
				require.NoError(t, err, "Failed to read spec file: %s", srcSpecPath)

				err = os.WriteFile(dstSpecPath, specContent, 0o644)
				require.NoError(t, err, "Failed to write spec file to SDK directory: %s", dstSpecPath)
			}
		}
	}
}

// setupCustomCodeTestDir creates a test directory in custom-code-test/speakeasy_tests
func setupCustomCodeTestDir(t *testing.T) string {
	t.Helper()

	baseDir := "/Users/ivangorshkov/speakeasy/repos/custom-code-test/speakeasy_tests"

	// Create base directory if it doesn't exist
	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err, "Failed to create base directory")

	// Create unique test directory
	testDir := filepath.Join(baseDir, fmt.Sprintf("test-%d", os.Getpid()))
	err = os.MkdirAll(testDir, 0o755)
	require.NoError(t, err, "Failed to create test directory")

	// Clean up after test
	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	return testDir
}
