package integration_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All supported targets from speakeasy generate supported-targets
var supportedTargets = []string{
	"mcp-typescript",
	"csharp",
	"go",
	"java",
	"php",
	"postman",
	"python",
	"ruby",
	// "typescript", // TODO: fix typescript quickstart - fails with defaultErrorName validation error
	"unity",
	"terraform",
}

func TestQuickstartMcpTypescript(t *testing.T) {
	testQuickstartTarget(t, "mcp-typescript")
}

func TestQuickstartCsharp(t *testing.T) {
	testQuickstartTarget(t, "csharp")
}

func TestQuickstartGo(t *testing.T) {
	testQuickstartTarget(t, "go")
}

func TestQuickstartJava(t *testing.T) {
	testQuickstartTarget(t, "java")
}

func TestQuickstartPhp(t *testing.T) {
	testQuickstartTarget(t, "php")
}

func TestQuickstartPostman(t *testing.T) {
	testQuickstartTarget(t, "postman")
}

func TestQuickstartPython(t *testing.T) {
	testQuickstartTarget(t, "python")
}

func TestQuickstartRuby(t *testing.T) {
	testQuickstartTarget(t, "ruby")
}

func TestQuickstartUnity(t *testing.T) {
	testQuickstartTarget(t, "unity")
}

func TestQuickstartTerraform(t *testing.T) {
	testQuickstartTarget(t, "terraform")
}

// testQuickstartTarget runs quickstart for a specific target with default options
func testQuickstartTarget(t *testing.T, target string) {
	t.Helper()

	// Create a completely isolated temporary directory
	tmpDir, err := os.MkdirTemp("", "speakeasy-quickstart-test-"+target+"-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to the temporary directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(originalDir)
		require.NoError(t, err)
	}()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Run quickstart with target flag and non-interactive mode for testing
	cmdErr := execute(t, tmpDir, "quickstart", "--target", target, "--non-interactive").Run()
	if cmdErr != nil {
		t.Fatalf("quickstart failed for target %s: %v", target, cmdErr)
	}

	// Verify workflow file was created
	workflowFile := filepath.Join(tmpDir, ".speakeasy", "workflow.yaml")
	assert.FileExists(t, workflowFile, "workflow.yaml should be created")

	// Verify openapi.yaml was created (sample spec)
	openapiFile := filepath.Join(tmpDir, "openapi.yaml")
	assert.FileExists(t, openapiFile, "openapi.yaml should be created")

	// Verify some target-specific files were created
	verifyTargetSpecificFiles(t, tmpDir, target)
}

// verifyTargetSpecificFiles checks for target-specific files that should be generated
func verifyTargetSpecificFiles(t *testing.T, targetDir, target string) {
	t.Helper()

	switch target {
	case "go":
		// Check for common Go files
		assert.FileExists(t, filepath.Join(targetDir, "go.mod"), "go.mod should be created for Go target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "mcp-typescript":
		// Check for TypeScript/Node.js files
		assert.FileExists(t, filepath.Join(targetDir, "package.json"), "package.json should be created for TypeScript target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "python":
		// Check for Python files
		assert.FileExists(t, filepath.Join(targetDir, "setup.py"), "setup.py should be created for Python target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "java":
		// Check for Java files
		assert.FileExists(t, filepath.Join(targetDir, "pom.xml"), "pom.xml should be created for Java target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "csharp":
		// Check for C# files
		files, err := filepath.Glob(filepath.Join(targetDir, "*.csproj"))
		require.NoError(t, err)
		assert.NotEmpty(t, files, "*.csproj file should be created for C# target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "php":
		// Check for PHP files
		assert.FileExists(t, filepath.Join(targetDir, "composer.json"), "composer.json should be created for PHP target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "ruby":
		// Check for Ruby files
		files, err := filepath.Glob(filepath.Join(targetDir, "*.gemspec"))
		require.NoError(t, err)
		assert.NotEmpty(t, files, "*.gemspec file should be created for Ruby target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "terraform":
		// Check for Terraform files
		assert.FileExists(t, filepath.Join(targetDir, "main.tf"), "main.tf should be created for Terraform target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "unity":
		// Check for Unity files
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	case "postman":
		// Check for Postman files
		files, err := filepath.Glob(filepath.Join(targetDir, "*.json"))
		require.NoError(t, err)
		assert.NotEmpty(t, files, "*.json file should be created for Postman target")
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
		
	default:
		// For any other targets, just check that README exists
		assert.FileExists(t, filepath.Join(targetDir, "README.md"), "README.md should be created")
	}
}