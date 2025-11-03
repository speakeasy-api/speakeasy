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

func TestCustomCode(t *testing.T) {
	t.Parallel()

	// Build the speakeasy binary once for all subtests
	speakeasyBinary := buildSpeakeasyBinaryOnce(t, "speakeasy-customcode-test-binary")

	t.Run("BasicWorkflow", func(t *testing.T) {
		t.Parallel()
		testCustomCodeBasicWorkflow(t, speakeasyBinary)
	})

	t.Run("ConflictResolution", func(t *testing.T) {
		t.Parallel()
		testCustomCodeConflictResolution(t, speakeasyBinary)
	})

	t.Run("ConflictResolutionAcceptOurs", func(t *testing.T) {
		t.Parallel()
		testCustomCodeConflictResolutionAcceptOurs(t, speakeasyBinary)
	})

	t.Run("SequentialPatchesAppliedWithRegenerationBetween", func(t *testing.T) {
		t.Parallel()
		testCustomCodeSequentialPatchesAppliedWithRegenerationBetween(t, speakeasyBinary)
	})

	t.Run("SequentialPatchesAppliedWithoutRegenerationBetween", func(t *testing.T) {
		t.Parallel()
		testCustomCodeSequentialPatchesAppliedWithoutRegenerationBetween(t, speakeasyBinary)
	})

	// t.Run("ConflictDetectionDuringCustomCodeRegistration", func(t *testing.T) {
	// 	t.Parallel()
	// 	testCustomCodeConflictDetectionDuringRegistration(t, speakeasyBinary)
	// })

	t.Run("NewFilePreservation", func(t *testing.T) {
		t.Parallel()
		testCustomCodeNewFilePreservation(t, speakeasyBinary)
	})

	t.Run("NewFileDeletion", func(t *testing.T) {
		t.Parallel()
		testCustomCodeNewFileDeletion(t, speakeasyBinary)
	})
}

// testCustomCodeBasicWorkflow tests basic custom code registration and reapplication
func testCustomCodeBasicWorkflow(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	httpMetadataPath := filepath.Join(temp, "models", "components", "httpmetadata.go")
	registerCustomCodeByPrefix(t, speakeasyBinary, temp, httpMetadataPath, "// Raw HTTP response", "\t// custom code")

	runRegeneration(t, speakeasyBinary, temp, true)
	verifyCustomCodePresent(t, httpMetadataPath, "// custom code")
}

// testCustomCodeConflictResolution tests conflict resolution workflow
func testCustomCodeConflictResolution(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	getUserByNamePath := filepath.Join(temp, "models", "operations", "getuserbyname.go")

	// Register custom code
	registerCustomCodeByPrefix(t, speakeasyBinary, temp, getUserByNamePath, "// The name that needs to be fetched", "\t// custom code")

	// Modify the spec to change line 477 from original description to "spec change"
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFile(t, specPath, 477, "        description: 'spec change'")

	// Run speakeasy run to regenerate - this should detect conflict and automatically enter resolution mode
	// The process should exit with code 2 after setting up conflict resolution
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Check for conflict markers in the file
	getUserByNameContent, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go")
	require.Contains(t, string(getUserByNameContent), "<<<<<<<", "File should contain conflict markers")

	// Resolve the conflict by accepting the patch (theirs)
	checkoutCmd := exec.Command("git", "checkout", "--theirs", getUserByNamePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --theirs should succeed: %s", string(checkoutOutput))

	// Stage the resolved file
	gitAddCmd := exec.Command("git", "add", getUserByNamePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Run customcode command again to register the resolved changes
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput))

	// Run speakeasy run again to verify patches are applied correctly
	runRegeneration(t, speakeasyBinary, temp, true)

	// Verify the custom code from the patch is present in the final file
	verifyCustomCodePresent(t, getUserByNamePath, "// custom code")
}

// testCustomCodeConflictResolutionAcceptOurs tests conflict resolution by accepting spec changes (ours)
func testCustomCodeConflictResolutionAcceptOurs(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	getUserByNamePath := filepath.Join(temp, "models", "operations", "getuserbyname.go")

	// Register custom code
	registerCustomCodeByPrefix(t, speakeasyBinary, temp, getUserByNamePath, "// The name that needs to be fetched", "\t// custom code")

	// Modify the spec to change line 477 from original description to "spec change"
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFile(t, specPath, 477, "        description: 'spec change'")

	// Run speakeasy run to regenerate - this should detect conflict and automatically enter resolution mode
	// The process should exit with code 2 after setting up conflict resolution
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Check for conflict markers in the file
	getUserByNameContent, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go")
	require.Contains(t, string(getUserByNameContent), "<<<<<<<", "File should contain conflict markers")

	// Resolve the conflict by accepting the spec changes (ours)
	checkoutCmd := exec.Command("git", "checkout", "--ours", getUserByNamePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --ours should succeed: %s", string(checkoutOutput))

	// Verify conflict markers are gone after checkout
	getUserByNameContentAfterCheckout, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go after checkout")
	require.NotContains(t, string(getUserByNameContentAfterCheckout), "<<<<<<<", "File should not contain conflict markers after checkout")

	// Stage the resolved file
	gitAddCmd := exec.Command("git", "add", getUserByNamePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Run customcode command again to register the resolved changes
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput))

	// Verify patch file was removed or is empty
	patchFile := filepath.Join(temp, ".speakeasy", "patches", "custom-code.diff")
	patchContent, err := os.ReadFile(patchFile)
	if err == nil {
		require.Empty(t, patchContent, "Patch file should be empty after accepting ours")
	}
	// If file doesn't exist, that's also fine

	// Verify gen.lock doesn't contain customCodeCommitHash
	genLockPath := filepath.Join(temp, ".speakeasy", "gen.lock")
	genLockContent, err := os.ReadFile(genLockPath)
	require.NoError(t, err, "Failed to read gen.lock")
	require.NotContains(t, string(genLockContent), "customCodeCommitHash", "gen.lock should not contain customCodeCommitHash after accepting ours")

	// Run speakeasy run again to verify patches are applied correctly
	runRegeneration(t, speakeasyBinary, temp, true)

	// Verify the spec change is present in the final file (not the custom code)
	finalContent, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go after final regeneration")
	require.Contains(t, string(finalContent), "spec change", "Spec change should be present after accepting ours")
	require.NotContains(t, string(finalContent), "// custom code", "Custom code should not be present after accepting ours")
}

// testCustomCodeSequentialPatchesAppliedWithRegenerationBetween tests that patches can be updated
// by registering a first patch, regenerating, then registering a second patch on the same line
func testCustomCodeSequentialPatchesAppliedWithRegenerationBetween(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	getUserByNamePath := filepath.Join(temp, "models", "operations", "getuserbyname.go")

	// Step 1: Register first patch
	registerCustomCodeByPrefix(t, speakeasyBinary, temp, getUserByNamePath, "// The name that needs to be fetched", "\t// first custom code")

	// Step 2: Verify first patch applies correctly on regeneration
	runRegeneration(t, speakeasyBinary, temp, true)
	verifyCustomCodePresent(t, getUserByNamePath, "// first custom code")

	// Step 2b: Commit the regenerated code with first patch applied
	gitCommit(t, temp, "regenerated with first patch")

	// Step 3: Modify the same line with different content (second patch)
	modifyLineInFileByPrefix(t, getUserByNamePath, "\t// first custom code", "\t// second custom code - updated")

	// Step 4: Register second patch (should update existing patch)
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed for second patch: %s", string(customCodeOutput))

	// Step 5: Verify patch file was updated (not appended)
	patchFile := filepath.Join(temp, ".speakeasy", "patches", "custom-code.diff")
	patchContent, err := os.ReadFile(patchFile)
	require.NoError(t, err, "Failed to read patch file")
	require.Contains(t, string(patchContent), "second custom code - updated", "Patch should contain second custom code")
	require.NotContains(t, string(patchContent), "first custom code", "Patch should not contain first custom code")

	// Step 6: Verify second patch applies correctly on final regeneration
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 7: Verify final file contains only second patch content
	finalContent, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go after final regeneration")
	require.Contains(t, string(finalContent), "// second custom code - updated", "File should contain second custom code")
	require.NotContains(t, string(finalContent), "// first custom code", "File should not contain first custom code")
}

// testCustomCodeSequentialPatchesAppliedWithoutRegenerationBetween tests that patches can be updated
// by registering a first patch, then immediately registering a second patch on the same line without regenerating
func testCustomCodeSequentialPatchesAppliedWithoutRegenerationBetween(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	getUserByNamePath := filepath.Join(temp, "models", "operations", "getuserbyname.go")

	// Step 1: Register first patch
	registerCustomCodeByPrefix(t, speakeasyBinary, temp, getUserByNamePath, "// The name that needs to be fetched", "\t// first custom code")

	// Step 2: Immediately modify the same line with different content (NO regeneration between)
	modifyLineInFileByPrefix(t, getUserByNamePath, "\t// first custom code", "\t// second custom code - updated")

	// Step 3: Register second patch (should update existing patch)
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed for second patch: %s", string(customCodeOutput))

	// Step 4: Verify patch file was updated (not appended)
	patchFile := filepath.Join(temp, ".speakeasy", "patches", "custom-code.diff")
	patchContent, err := os.ReadFile(patchFile)
	require.NoError(t, err, "Failed to read patch file")
	require.Contains(t, string(patchContent), "second custom code - updated", "Patch should contain second custom code")
	require.NotContains(t, string(patchContent), "first custom code", "Patch should not contain first custom code")

	// Step 5: Verify second patch applies correctly on regeneration
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 6: Verify final file contains only second patch content
	finalContent, err := os.ReadFile(getUserByNamePath)
	require.NoError(t, err, "Failed to read getuserbyname.go after final regeneration")
	require.Contains(t, string(finalContent), "// second custom code - updated", "File should contain second custom code")
	require.NotContains(t, string(finalContent), "// first custom code", "File should not contain first custom code")
}

// buildSpeakeasyBinaryOnce builds the speakeasy binary and returns the path to it
func buildSpeakeasyBinaryOnce(t *testing.T, binaryName string) string {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	binaryPath := filepath.Join(baseFolder, binaryName)

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

func modifyLineInFileByPrefix(t *testing.T, filePath string, oldContentPrefix string, newContent string) {
	t.Helper()

	lineNum := findLineNumberByPrefix(t, filePath, oldContentPrefix)
	modifyLineInFile(t, filePath, lineNum, newContent)
}

// setupCustomCodeTestDir creates a test directory outside the speakeasy repo
func setupCustomCodeTestDir(t *testing.T) string {
	t.Helper()

	// Check for custom test directory environment variable
	baseDir := os.Getenv("SPEAKEASY_TEST_DIR")
	if baseDir == "" {
		// Fall back to system temp
		baseDir = os.TempDir()
	}

	// Create unique test directory
	testDir, err := os.MkdirTemp(baseDir, "speakeasy-customcode-*")
	require.NoError(t, err, "Failed to create test directory")

	// Clean up after test
	// t.Cleanup(func() {
	// 	os.RemoveAll(testDir)
	// })

	return testDir
}

// setupSDKGeneration sets up a test directory with SDK generation and git initialization
func setupSDKGeneration(t *testing.T, speakeasyBinary, inputDoc string) string {
	t.Helper()

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
				Location: workflow.LocationString(inputDoc),
			},
		},
	}

	// Single go target with no output directory - generates to workspace root
	target := workflow.Target{
		Target: "go",
		Source: "first-source",
		// Output: nil - generates directly in workspace root
	}
	workflowFile.Targets["test-target"] = target

	if isLocalFileReference(inputDoc) {
		err := copyFile("resources/customcodespec.yaml", fmt.Sprintf("%s/%s", temp, inputDoc))
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

	// Run go mod tidy to ensure go.sum is properly populated
	// This is necessary because we used --skip-compile above
	goModTidyCmd := exec.Command("go", "mod", "tidy")
	goModTidyCmd.Dir = temp
	output, err := goModTidyCmd.CombinedOutput()
	require.NoError(t, err, "Failed to run go mod tidy: %s", string(output))

	// SDK is generated in workspace root
	checkForExpectedFiles(t, temp, expectedFilesByLanguage("go"))

	// Initialize git repository in the workspace root
	initGitRepo(t, temp)

	// Commit all generated files with "clean generation" message
	gitCommit(t, temp, "clean generation")

	// Verify the commit was created with the correct message
	verifyGitCommit(t, temp, "clean generation")

	return temp
}

// findLineNumberByPrefix finds the line number (1-indexed) of the first line containing the prefix
func findLineNumberByPrefix(t *testing.T, filePath, prefix string) int {
	t.Helper()

	file, err := os.Open(filePath)
	require.NoError(t, err, "Failed to open file: %s", filePath)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 1
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), prefix) {
			return lineNumber
		}
		lineNumber++
	}
	require.NoError(t, scanner.Err(), "Failed to read file: %s", filePath)
	require.Fail(t, "Could not find line with prefix: %s", prefix)
	return -1 // Never reached
}

// registerCustomCodeByPrefix finds a line by prefix and registers custom code at that line
func registerCustomCodeByPrefix(t *testing.T, speakeasyBinary, workingDir, filePath, linePrefix string, newContent string) {
	t.Helper()

	lineNum := findLineNumberByPrefix(t, filePath, linePrefix)
	registerCustomCode(t, speakeasyBinary, workingDir, filePath, lineNum, newContent)
}

// registerCustomCode modifies a file and registers it as custom code
func registerCustomCode(t *testing.T, speakeasyBinary, workingDir, filePath string, lineNum int, newContent string) {
	t.Helper()

	// Modify the file
	modifyLineInFile(t, filePath, lineNum, newContent)

	// Run customcode command
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = workingDir
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Verify patches directory was created
	patchesDir := filepath.Join(workingDir, ".speakeasy", "patches")
	_, err := os.Stat(patchesDir)
	require.NoError(t, err, "patches directory should exist at %s", patchesDir)

	// Verify patch file was created
	patchFile := filepath.Join(patchesDir, "custom-code.diff")
	_, err = os.Stat(patchFile)
	require.NoError(t, err, "patch file should exist at %s", patchFile)
}

// runRegeneration runs speakeasy run and checks if it succeeds or fails based on expectSuccess
func runRegeneration(t *testing.T, speakeasyBinary, workingDir string, expectSuccess bool) {
	t.Helper()

	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = workingDir
	regenOutput, regenErr := regenCmd.CombinedOutput()

	if expectSuccess {
		require.NoError(t, regenErr, "speakeasy run should succeed on regeneration: %s", string(regenOutput))
	} else {
		require.Error(t, regenErr, "speakeasy run should fail due to conflicts: %s", string(regenOutput))
		require.Contains(t, string(regenOutput), "conflict", "Output should mention conflicts")
	}
}

// verifyCustomCodePresent checks that custom code is present in the specified file
func verifyCustomCodePresent(t *testing.T, filePath, expectedContent string) {
	t.Helper()

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read file: %s", filePath)
	require.Contains(t, string(content), expectedContent, "Custom code should be present in file")
}

// testCustomCodeNewFilePreservation tests that custom code registration preserves entirely new files
func testCustomCodeNewFilePreservation(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Create a new file with helper functions
	helperFilePath := filepath.Join(temp, "utils", "helper.go")
	helperFileContent := `package utils

import "fmt"

// FormatUserID formats a user ID with a prefix
func FormatUserID(id int64) string {
	return fmt.Sprintf("user_%d", id)
}

// ValidateUserID validates that a user ID is positive
func ValidateUserID(id int64) bool {
	return id > 0
}
`

	// Create the utils directory
	err := os.MkdirAll(filepath.Join(temp, "utils"), 0o755)
	require.NoError(t, err, "Failed to create utils directory")

	// Write the helper file
	err = os.WriteFile(helperFilePath, []byte(helperFileContent), 0o644)
	require.NoError(t, err, "Failed to write helper file")

	// Stage the new file so git diff HEAD can capture it
	gitAddCmd := exec.Command("git", "add", helperFilePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Register custom code
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Verify patch file was created
	patchFile := filepath.Join(temp, ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(patchFile)
	require.NoError(t, err, "patch file should exist at %s", patchFile)

	// Verify the file exists after registration (before regeneration)
	_, err = os.Stat(helperFilePath)
	require.NoError(t, err, "Helper file should exist after registration")

	// Run speakeasy run to regenerate the SDK
	// This should apply the patch and preserve the new file
	runRegeneration(t, speakeasyBinary, temp, true)

	// Verify the new file still exists after regeneration
	_, err = os.Stat(helperFilePath)
	require.NoError(t, err, "Helper file should exist after regeneration")

	// Verify the file contents are preserved exactly
	verifyCustomCodePresent(t, helperFilePath, "FormatUserID")
	verifyCustomCodePresent(t, helperFilePath, "ValidateUserID")
	verifyCustomCodePresent(t, helperFilePath, "package utils")

	// Read the entire file and verify exact content match
	actualContent, err := os.ReadFile(helperFilePath)
	require.NoError(t, err, "Failed to read helper file after regeneration")
	require.Equal(t, helperFileContent, string(actualContent), "Helper file content should be preserved exactly")
}

// testCustomCodeNewFileDeletion tests that deleting a custom file is properly registered and persisted
func testCustomCodeNewFileDeletion(t *testing.T, speakeasyBinary string) {
	temp := setupSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Create a new file with helper functions
	helperFilePath := filepath.Join(temp, "utils", "helper.go")
	helperFileContent := `package utils

import "fmt"

// FormatUserID formats a user ID with a prefix
func FormatUserID(id int64) string {
	return fmt.Sprintf("user_%d", id)
}
`

	// Create the utils directory
	err := os.MkdirAll(filepath.Join(temp, "utils"), 0o755)
	require.NoError(t, err, "Failed to create utils directory")

	// Write the helper file
	err = os.WriteFile(helperFilePath, []byte(helperFileContent), 0o644)
	require.NoError(t, err, "Failed to write helper file")

	// Stage the new file so git diff HEAD can capture it
	gitAddCmd := exec.Command("git", "add", helperFilePath)
	gitAddCmd.Dir = temp
	_, err = gitAddCmd.CombinedOutput()
	require.NoError(t, err, "git add should succeed")

	// Register custom code (registers the new file)
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Verify patch file was created
	patchFile := filepath.Join(temp, ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(patchFile)
	require.NoError(t, err, "patch file should exist after registering new file")

	// Regenerate and verify the file is preserved
	runRegeneration(t, speakeasyBinary, temp, true)
	_, err = os.Stat(helperFilePath)
	require.NoError(t, err, "Helper file should exist after first regeneration")

	// Commit the regeneration so the file becomes part of HEAD
	gitCommitCmd := exec.Command("git", "commit", "-am", "regeneration with custom file")
	gitCommitCmd.Dir = temp
	_, err = gitCommitCmd.CombinedOutput()
	require.NoError(t, err, "git commit should succeed after regeneration")

	// Now delete the file
	err = os.Remove(helperFilePath)
	require.NoError(t, err, "Failed to delete helper file")

	// Register the deletion
	customCodeCmd = exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr = customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed after deletion: %s", string(customCodeOutput))

	// Verify patch file was removed (no custom code remaining)
	_, err = os.Stat(patchFile)
	require.True(t, os.IsNotExist(err), "patch file should not exist after deleting the only custom file")

	// Regenerate and verify the file remains deleted
	runRegeneration(t, speakeasyBinary, temp, true)
	_, err = os.Stat(helperFilePath)
	require.True(t, os.IsNotExist(err), "Helper file should not exist after regeneration with deletion registered")
}
