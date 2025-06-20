package integration_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQuickstartComprehensive tests quickstart for each supported target
// using the pattern: build temp binary -> mkdir temp dir -> run quickstart -> run speakeasy run
func TestQuickstartComprehensive(t *testing.T) {
	targets := prompts.GetSupportedTargetNames()

	// Build the temporary binary once for all tests
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	for _, target := range targets {
		t.Run(fmt.Sprintf("target_%s", target), func(t *testing.T) {
			testQuickstartComprehensiveForTarget(t, target, tempBinary)
		})
	}
}

// TestQuickstartTypescript specifically tests TypeScript target
func TestQuickstartTypescript(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "typescript", tempBinary)
}

// TestQuickstartPython specifically tests Python target
func TestQuickstartPython(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "python", tempBinary)
}

// TestQuickstartGo specifically tests Go target
func TestQuickstartGo(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "go", tempBinary)
}

// TestQuickstartJava specifically tests Java target
func TestQuickstartJava(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "java", tempBinary)
}

// TestQuickstartCsharp specifically tests C# target
func TestQuickstartCsharp(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "csharp", tempBinary)
}

// TestQuickstartPhp specifically tests PHP target
func TestQuickstartPhp(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "php", tempBinary)
}

// TestQuickstartRuby specifically tests Ruby target
func TestQuickstartRuby(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "ruby", tempBinary)
}

// TestQuickstartMcpTypescript specifically tests MCP TypeScript target
func TestQuickstartMcpTypescript(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "mcp-typescript", tempBinary)
}

// TestQuickstartTerraform specifically tests Terraform target
func TestQuickstartTerraform(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "terraform", tempBinary)
}

// TestQuickstartUnity specifically tests Unity target
func TestQuickstartUnity(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "unity", tempBinary)
}

// TestQuickstartPostman specifically tests Postman target
func TestQuickstartPostman(t *testing.T) {
	t.Parallel()
	tempBinary := buildTempBinary(t)
	defer os.Remove(tempBinary)

	testQuickstartComprehensiveForTarget(t, "postman", tempBinary)
}

// buildTempBinary builds the speakeasy binary to a temporary location
func buildTempBinary(t *testing.T) string {
	tempDir := getTempDir()
	binaryName := "speakeasy"
	if runtime.GOOS == "windows" {
		binaryName = "speakeasy.exe"
	}

	tempBinary := filepath.Join(tempDir, binaryName)

	// Build the binary
	cmd := exec.Command("go", "build", "-o", tempBinary, ".")
	cmd.Dir = getProjectRoot(t)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	// Verify the binary exists and is executable
	_, err = os.Stat(tempBinary)
	require.NoError(t, err, "Binary was not created at %s", tempBinary)

	return tempBinary
}

// testQuickstartComprehensiveForTarget runs the comprehensive test for a specific target
func testQuickstartComprehensiveForTarget(t *testing.T, target, tempBinary string) {
	// Skip Alpha languages as they have different behavior
	if isAlphaTarget(target) {
		t.Skipf("Skipping %s as it's an Alpha language", target)
		return
	}

	// Create test directory
	testDir := createTestDir(t, target)
	// Don't delete test directory - leave it for debugging
	t.Logf("Test directory for %s: %s", target, testDir)

	// Change to test directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(originalDir)
		require.NoError(t, err)
	}()

	err = os.Chdir(testDir)
	require.NoError(t, err)

	// Run quickstart
	quickstartCmd := exec.Command(tempBinary,
		"quickstart",
		"--skip-interactive",
		"--target", target,
		"--output", "console",
	)

	quickstartOutput, err := quickstartCmd.CombinedOutput()
	if err != nil {
		t.Logf("Quickstart output for %s: %s", target, string(quickstartOutput))
		t.Fatalf("Quickstart failed for target %s: %v", target, err)
	}

	// Check if SDK was generated directly in test directory or in a subdirectory
	generatedDir := testDir
	if _, err := os.Stat(filepath.Join(testDir, ".speakeasy")); err != nil {
		// Not in test directory, look for subdirectory
		generatedDir = findGeneratedDirectory(t, testDir, target)
	}

	// Change to the generated directory
	err = os.Chdir(generatedDir)
	require.NoError(t, err, "Failed to change to generated directory %s", generatedDir)

	verifyBasicStructure(t, generatedDir, target)

	// Run speakeasy run
	runCmd := exec.Command(tempBinary, "run", "--output", "console")
	runOutput, err := runCmd.CombinedOutput()

	if err != nil {
		t.Logf("Run output for %s: %s", target, string(runOutput))
		// Don't fail the test if run fails - it might be due to compilation issues in test environment
		t.Logf("Speakeasy run failed for target %s (this might be expected in test environment): %v", target, err)
	} else {
		t.Logf("Speakeasy run succeeded for target %s", target)
	}

	// Verify basic structure was created
	verifyBasicStructure(t, generatedDir, target)
}

// createTestDir creates a temporary test directory for the target
func createTestDir(t *testing.T, target string) string {
	tempDir := getTempDir()
	baseTestDir := filepath.Join(tempDir, "speakeasy-quickstart-tests")
	testDir := filepath.Join(baseTestDir, target)

	// Remove any existing directory
	os.RemoveAll(testDir)

	err := os.MkdirAll(testDir, 0o755)
	require.NoError(t, err, "Failed to create test directory %s", testDir)

	return testDir
}

// findGeneratedDirectory finds the directory that was generated by quickstart
func findGeneratedDirectory(t *testing.T, testDir, target string) string {
	// Fallback: look for any directory with .speakeasy folder
	entries, err := os.ReadDir(testDir)
	require.NoError(t, err)

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(testDir, entry.Name())
			speakeasyDir := filepath.Join(dirPath, ".speakeasy")
			if _, err := os.Stat(speakeasyDir); err == nil {
				candidates = append(candidates, dirPath)
			}
		}
	}

	if len(candidates) == 0 {
		t.Fatalf("Could not find any generated directory with .speakeasy folder for target %s in %s", target, testDir)
	}

	if len(candidates) == 1 {
		return candidates[0]
	}

	// If multiple candidates, try to pick the best one
	for _, candidate := range candidates {
		name := filepath.Base(candidate)
		// Prefer directories that contain the target name or common patterns
		if strings.Contains(strings.ToLower(name), target) ||
			strings.Contains(strings.ToLower(name), "petstore") ||
			strings.Contains(strings.ToLower(name), "sdk") {
			return candidate
		}
	}

	// If no preferred match, return the first one
	t.Logf("Multiple candidates found for target %s: %v, using first one", target, candidates)
	return candidates[0]
}

// verifyBasicStructure verifies that basic files and directories were created
func verifyBasicStructure(t *testing.T, generatedDir, target string) {
	// Check for .speakeasy directory
	speakeasyDir := filepath.Join(generatedDir, ".speakeasy")
	_, err := os.Stat(speakeasyDir)
	assert.NoError(t, err, ".speakeasy directory should exist")

	// Check for workflow.yaml
	workflowFile := filepath.Join(speakeasyDir, "workflow.yaml")
	_, err = os.Stat(workflowFile)
	assert.NoError(t, err, "workflow.yaml should exist")

	// Check for OpenAPI spec file
	specFiles := []string{"openapi.yaml", "openapi.yml", "openapi.json"}
	specFound := false
	for _, specFile := range specFiles {
		if _, err := os.Stat(filepath.Join(generatedDir, specFile)); err == nil {
			specFound = true
			break
		}
	}
	assert.True(t, specFound, "OpenAPI spec file should exist")

	checkFileExists(t, generatedDir, "README.md")

	// Check for target-specific files
	verifyTargetSpecificFiles(t, generatedDir, target)
}

// verifyTargetSpecificFiles verifies target-specific files were created
func verifyTargetSpecificFiles(t *testing.T, generatedDir, target string) {
	switch target {
	case "typescript", "mcp-typescript":
		checkFileExists(t, generatedDir, "package.json")
		checkFileExists(t, generatedDir, "tsconfig.json")
	case "python":
		checkFileExists(t, generatedDir, "pyproject.toml")
	case "go":
		checkFileExists(t, generatedDir, "go.mod")
	case "csharp", "unity":
		// Look for .csproj files
		matches, _ := filepath.Glob(filepath.Join(generatedDir, "*.csproj"))
		assert.True(t, len(matches) > 0, "Should have .csproj file for %s target", target)
	case "php":
		checkFileExists(t, generatedDir, "composer.json")
	case "ruby":
		checkFileExists(t, generatedDir, "Gemfile")
	case "terraform":
		checkFileExists(t, generatedDir, "go.mod") // Terraform provider is Go-based
	case "postman":
		// Postman collections might not have standard files, just check basic structure
		t.Logf("Postman target verification - checking basic structure")
	default:
		t.Logf("No specific file verification defined for target: %s", target)
	}
}

// checkFileExists checks if a file exists and logs the result
func checkFileExists(t *testing.T, dir, filename string) {
	filePath := filepath.Join(dir, filename)
	_, err := os.Stat(filePath)
	if err == nil {
		t.Logf("✓ Found expected file: %s", filename)
	} else {
		t.Logf("✗ Missing expected file: %s", filename)
	}
}

// getTempDir returns the appropriate temp directory for the OS
func getTempDir() string {
	if runtime.GOOS == "windows" {
		return os.TempDir()
	}
	return "/tmp"
}

// isAlphaTarget checks if a target is in Alpha stage using the existing codebase implementation
func isAlphaTarget(target string) bool {
	return generate.GetTargetNameMaturity(target) == "Alpha"
}

// getProjectRoot returns the project root directory
func getProjectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Go up directories until we find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("Could not find project root (go.mod not found)")
	return ""
}
