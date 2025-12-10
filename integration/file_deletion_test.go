package integration_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

// TestFileDeletion_UnusedModelRemoved verifies that when a model is no longer used in the OpenAPI spec,
// the generated model file is deleted on regeneration.
func TestFileDeletion_UnusedModelRemoved(t *testing.T) {
	t.Parallel()
	temp := setupFileDeletionTestDir(t)

	// Initial generation with both Pet and Owner models
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")

	// Commit initial generation
	gitCommitAll(t, temp, "initial generation")

	// Verify both model files exist
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	ownerFile := filepath.Join(temp, "models", "components", "owner.go")

	require.FileExists(t, petFile, "Pet model file should exist after initial generation")
	require.FileExists(t, ownerFile, "Owner model file should exist after initial generation")

	t.Logf("Initial generation complete. Pet file: %s, Owner file: %s", petFile, ownerFile)

	// List all generated files for debugging
	t.Log("Generated files after initial generation:")
	listGeneratedFiles(t, temp)

	// Dump gen.lock after first generation
	genLockPath := filepath.Join(temp, ".speakeasy", "gen.lock")
	if content, err := os.ReadFile(genLockPath); err == nil {
		t.Logf("gen.lock after initial generation:\n%s", string(content))
	} else {
		t.Logf("Could not read gen.lock: %v", err)
	}

	// Now update the spec to remove the Owner model usage
	specWithoutOwner := `openapi: 3.0.3
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
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specWithoutOwner), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: remove Owner model")

	// Regenerate
	t.Log("Regenerating without Owner model...")
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Regeneration should succeed")

	// List generated files after regeneration for debugging
	t.Log("Generated files after regeneration:")
	listGeneratedFiles(t, temp)

	// Dump gen.lock after regeneration
	if content, err := os.ReadFile(genLockPath); err == nil {
		t.Logf("gen.lock after regeneration:\n%s", string(content))
	} else {
		t.Logf("Could not read gen.lock: %v", err)
	}

	// Pet model should still exist
	require.FileExists(t, petFile, "Pet model file should still exist after regeneration")

	// Owner model should be DELETED since it's no longer used
	_, err = os.Stat(ownerFile)
	require.True(t, os.IsNotExist(err), "Owner model file should be deleted when no longer used in spec. Error: %v", err)
}

// setupFileDeletionTestDir creates a test directory with an OpenAPI spec that uses two models
func setupFileDeletionTestDir(t *testing.T) string {
	t.Helper()

	temp := setupTestDir(t)

	// Create an OpenAPI spec with TWO models: Pet and Owner
	// Both are used by the /pets endpoint
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
  /owners:
    get:
      summary: List owners
      operationId: listOwners
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Owner'
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
    Owner:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
        email:
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

	// Create gen.yaml
	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
go:
  version: 1.0.0
  packageName: testsdk
`
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Create .genignore to exclude go.mod/go.sum from generation
	genignoreContent := `go.mod
go.sum
`
	err = os.WriteFile(filepath.Join(temp, ".genignore"), []byte(genignoreContent), 0644)
	require.NoError(t, err)

	// Initialize git repo
	gitInit(t, temp)
	gitCommitAll(t, temp, "initial commit")

	return temp
}

// listGeneratedFiles lists all generated files for debugging purposes
func listGeneratedFiles(t *testing.T, dir string) {
	t.Helper()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories
		if info.IsDir() && info.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(dir, path)
			t.Logf("  %s", relPath)
		}
		return nil
	})
	if err != nil {
		t.Logf("Error walking directory: %v", err)
	}
}
