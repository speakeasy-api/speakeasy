package integration_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestAutomaticSwaggerConversion(t *testing.T) {
	t.Parallel()
	temp := t.TempDir()

	// Create workflow file that uses the Swagger 2.0 document
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: make(map[string]workflow.Source),
		Targets: make(map[string]workflow.Target),
	}

	// Copy the Swagger 2.0 test file
	swaggerPath := "swagger.yaml"
	err := copyFile("resources/swagger.yaml", filepath.Join(temp, swaggerPath))
	require.NoError(t, err)

	outputPath := filepath.Join(temp, "output.yaml")
	workflowFile.Sources["swagger-source"] = workflow.Source{
		Inputs: []workflow.Document{
			{
				Location: workflow.LocationString(swaggerPath),
			},
		},
		Output: &outputPath,
	}

	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	args := []string{"run", "-s", "all", "--pinned", "--skip-compile"}
	cmdErr := execute(t, temp, args...).Run()
	require.NoError(t, cmdErr)

	// Read the output file and verify it was converted to OpenAPI 3.0
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err, "No readable file %s exists", outputPath)

	contentStr := string(content)

	// Verify it's OpenAPI 3.0, not Swagger 2.0
	require.Contains(t, contentStr, "openapi: 3.", "Output should be OpenAPI 3.x")
	require.NotContains(t, contentStr, "swagger: \"2.0\"", "Output should not contain Swagger 2.0 declaration")
	require.NotContains(t, contentStr, "swagger: '2.0'", "Output should not contain Swagger 2.0 declaration")

	// Verify some paths from the original Swagger doc are preserved
	require.True(t, strings.Contains(contentStr, "/pet") ||
		strings.Contains(contentStr, "\"/pet\""), "Should contain /pet path")
	require.True(t, strings.Contains(contentStr, "/store/inventory") ||
		strings.Contains(contentStr, "\"/store/inventory\""), "Should contain /store/inventory path")
}
