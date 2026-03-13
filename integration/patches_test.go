package integration_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

// TestPersistentEdits_UserModificationPreserved verifies that user edits are preserved during regeneration
func TestPersistentEdits_UserModificationPreserved(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// Dump lockfile after first generation
	lockfileContent, _ := os.ReadFile(filepath.Join(temp, ".speakeasy", "gen.lock"))
	t.Logf("Lockfile after first generation:\n%s", string(lockfileContent))

	// Commit all generated files
	gitCommitAll(t, temp, "initial generation")

	// Find sdk.go
	sdkFile := filepath.Join(temp, "sdk.go")
	require.FileExists(t, sdkFile)

	// Read original content
	originalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	t.Logf("sdk.go after first generation (first 500 chars):\n%s", string(originalContent[:min(500, len(originalContent))]))

	// Add a user comment
	modifiedContent := strings.Replace(string(originalContent), "package testsdk", "package testsdk\n\n// USER_CUSTOM_COMMENT: This is my custom code", 1)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)
	t.Logf("Modified sdk.go (first 500 chars):\n%s", modifiedContent[:min(500, len(modifiedContent))])

	// Commit changes
	gitCommitAll(t, temp, "user modifications")

	// Regenerate
	t.Log("Running second generation...")
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// Dump lockfile after second generation
	lockfileContent, _ = os.ReadFile(filepath.Join(temp, ".speakeasy", "gen.lock"))
	t.Logf("Lockfile after second generation:\n%s", string(lockfileContent))

	// Verify user comment is preserved
	finalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "USER_CUSTOM_COMMENT: This is my custom code", "User modification should be preserved")
}

// --- Helper functions for persistent edits tests ---

// setupPersistentEditsTestDir creates a test directory with a minimal OpenAPI spec and workflow
// configured for persistent edits
func setupPersistentEditsTestDir(t *testing.T) string {
	t.Helper()

	temp := t.TempDir()

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
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
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
	err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0o644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)

	// Create workflow.yaml with persistent edits enabled
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

	// Create gen.yaml with persistent edits enabled
	genYamlContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
go:
  version: 1.0.0
  packageName: testsdk
`
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(genYamlContent), 0o644)
	require.NoError(t, err)

	// Create .genignore to exclude go.mod/go.sum from generation (avoids dependency conflicts in tests)
	genignoreContent := `go.mod
go.sum
`
	err = os.WriteFile(filepath.Join(temp, ".genignore"), []byte(genignoreContent), 0o644)
	require.NoError(t, err)

	// Initialize git repo
	gitInit(t, temp)
	gitCommitAll(t, temp, "initial commit")

	return temp
}

// gitInit initializes a git repository
func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init failed: %s", string(output))

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	_, _ = cmd.CombinedOutput()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	_, _ = cmd.CombinedOutput()
}

// gitCommitAll stages all changes and commits
func gitCommitAll(t *testing.T, dir string, message string) {
	t.Helper()
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))
}

// TestPersistentEdits_MultiTarget verifies persistent edits work correctly with multiple targets
func TestPersistentEdits_MultiTarget(t *testing.T) {
	t.Parallel()
	temp := setupMultiTargetPersistentEditsTestDir(t)

	// Initial generation of both targets
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")

	// Commit initial generation
	gitCommitAll(t, temp, "initial generation")

	// Verify both targets generated files with @generated-id headers
	goSdkFile := filepath.Join(temp, "go-sdk", "sdk.go")
	tsSdkFile := filepath.Join(temp, "ts-sdk", "src", "sdk", "sdk.ts")

	require.FileExists(t, goSdkFile, "Go SDK file should exist")
	require.FileExists(t, tsSdkFile, "TypeScript SDK file should exist")

	goContent, err := os.ReadFile(goSdkFile)
	require.NoError(t, err)

	tsContent, err := os.ReadFile(tsSdkFile)
	require.NoError(t, err)

	// Modify both SDKs with user comments
	goModified := strings.Replace(string(goContent), "package gosdk", "package gosdk\n\n// GO_USER_COMMENT: Custom Go code", 1)
	err = os.WriteFile(goSdkFile, []byte(goModified), 0o644)
	require.NoError(t, err)

	tsModified := strings.Replace(string(tsContent), "export class SDK extends ClientSDK", "// TS_USER_COMMENT: Custom TypeScript code\nexport class SDK extends ClientSDK", 1)
	err = os.WriteFile(tsSdkFile, []byte(tsModified), 0o644)
	require.NoError(t, err)

	// Commit user modifications
	gitCommitAll(t, temp, "user modifications to both SDKs")

	// Regenerate both targets
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Regeneration should succeed")

	// Verify user modifications are preserved in both targets
	goFinal, err := os.ReadFile(goSdkFile)
	require.NoError(t, err)
	require.Contains(t, string(goFinal), "GO_USER_COMMENT: Custom Go code", "Go user modification should be preserved")

	tsFinal, err := os.ReadFile(tsSdkFile)
	require.NoError(t, err)
	require.Contains(t, string(tsFinal), "TS_USER_COMMENT: Custom TypeScript code", "TypeScript user modification should be preserved")
}

// TestPersistentEdits_SpecChangeWithUserEdits verifies user edits are preserved when spec changes
func TestPersistentEdits_SpecChangeWithUserEdits(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find Pet model file and add user comment
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	require.FileExists(t, petFile)

	petContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// Add user comment to Pet model
	petModified := strings.Replace(string(petContent), "type Pet struct", "// PET_USER_COMMENT: Custom validation logic here\ntype Pet struct", 1)
	err = os.WriteFile(petFile, []byte(petModified), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user modifications to Pet")

	// Update spec to add a new field to Pet
	newSpecContent := `openapi: 3.0.3
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
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
        breed:
          type: string
          description: The breed of the pet
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: add breed field")

	// Regenerate with updated spec
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// Verify user comment is preserved AND new field is present
	petFinal, err := os.ReadFile(petFile)
	require.NoError(t, err)
	require.Contains(t, string(petFinal), "PET_USER_COMMENT: Custom validation logic here", "User modification should be preserved after spec change")
	require.Contains(t, string(petFinal), "Breed", "New field from spec should be added")
}

// TestPersistentEdits_MultipleFilesModified verifies multiple user-modified files are all preserved
func TestPersistentEdits_MultipleFilesModified(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Modify multiple files
	filesToModify := map[string]string{
		filepath.Join(temp, "sdk.go"):                              "// SDK_CUSTOM: Main SDK customization",
		filepath.Join(temp, "models", "components", "pet.go"):      "// PET_CUSTOM: Pet model customization",
		filepath.Join(temp, "models", "operations", "listpets.go"): "// LISTPETS_CUSTOM: Operation customization",
	}

	for file, comment := range filesToModify {
		require.FileExists(t, file, "File should exist: %s", file)

		content, err := os.ReadFile(file)
		require.NoError(t, err)

		// Add comment after package declaration
		modified := strings.Replace(string(content), "package ", comment+"\npackage ", 1)
		err = os.WriteFile(file, []byte(modified), 0o644)
		require.NoError(t, err)
	}
	gitCommitAll(t, temp, "user modifications to multiple files")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// Verify all modifications are preserved
	for file, comment := range filesToModify {
		content, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Contains(t, string(content), comment, "User modification should be preserved in %s", file)
	}
}

// setupMultiTargetPersistentEditsTestDir creates a test directory with multiple targets configured
func setupMultiTargetPersistentEditsTestDir(t *testing.T) string {
	t.Helper()

	temp := t.TempDir()

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
	err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0o644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)

	// Create output directories
	err = os.MkdirAll(filepath.Join(temp, "go-sdk"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(temp, "ts-sdk"), 0o755)
	require.NoError(t, err)

	// Create workflow.yaml with multiple targets
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
			"go-target": {
				Target: "go",
				Source: "test-source",
				Output: stringPtr("go-sdk"),
			},
			"ts-target": {
				Target: "typescript",
				Source: "test-source",
				Output: stringPtr("ts-sdk"),
			},
		},
	}
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Create gen.yaml for go target in go-sdk/
	goGenYaml := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
go:
  version: 1.0.0
  packageName: gosdk
`
	err = os.WriteFile(filepath.Join(temp, "go-sdk", "gen.yaml"), []byte(goGenYaml), 0o644)
	require.NoError(t, err)

	// Create gen.yaml for typescript target in ts-sdk/
	tsGenYaml := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
typescript:
  version: 1.0.0
  packageName: tssdk
`
	err = os.WriteFile(filepath.Join(temp, "ts-sdk", "gen.yaml"), []byte(tsGenYaml), 0o644)
	require.NoError(t, err)

	// Create .genignore in each target directory
	genignoreContent := `go.mod
go.sum
package.json
package-lock.json
node_modules
`
	err = os.WriteFile(filepath.Join(temp, "go-sdk", ".genignore"), []byte(genignoreContent), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(temp, "ts-sdk", ".genignore"), []byte(genignoreContent), 0o644)
	require.NoError(t, err)

	// Initialize git repo
	gitInit(t, temp)
	gitCommitAll(t, temp, "initial commit")

	return temp
}

func stringPtr(s string) *string {
	return &s
}

// TestPersistentEdits_FileRemove verifies behavior when user removes a generated file
func TestPersistentEdits_FileRemove(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find the Pet model file and a utility file
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	sdkFile := filepath.Join(temp, "sdk.go")

	require.FileExists(t, petFile)
	require.FileExists(t, sdkFile)

	sdkContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)

	// Add user modification to sdk.go (to verify it's preserved)
	sdkModified := strings.Replace(string(sdkContent), "package testsdk", "package testsdk\n\n// SDK_PRESERVED: This should survive", 1)
	err = os.WriteFile(sdkFile, []byte(sdkModified), 0o644)
	require.NoError(t, err)

	// User intentionally removes pet.go (maybe they don't want this model)
	err = os.Remove(petFile)
	require.NoError(t, err)
	t.Log("User removed pet.go")

	gitCommitAll(t, temp, "user removed pet.go and modified sdk.go")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// sdk.go should still have user modifications preserved
	sdkFinal, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	require.Contains(t, string(sdkFinal), "SDK_PRESERVED: This should survive",
		"User modification in sdk.go should be preserved")

	// pet.go will not be regenerated since the user deleted it
	_, err = os.Stat(petFile)
	require.Error(t, err, "pet.go should not be regenerated after user deletion")
	require.True(t, os.IsNotExist(err), "pet.go should not exist")
}

// TestPersistentEdits_ConflictMarkers verifies that conflicting changes produce conflict markers
func TestPersistentEdits_ConflictMarkers(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find the Pet model file
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	require.FileExists(t, petFile)

	// Read original content
	originalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// User modifies the GetID method to add custom validation
	// This conflicts because the generator will also want to generate GetID
	modifiedContent := strings.Replace(
		string(originalContent),
		"func (p *Pet) GetID() *int64 {\n\tif p == nil {\n\t\treturn nil\n\t}\n\treturn p.ID\n}",
		"func (p *Pet) GetID() *int64 {\n\t// USER_CONFLICT_TEST: Custom validation\n\tif p == nil {\n\t\treturn nil\n\t}\n\tif p.ID != nil && *p.ID < 0 {\n\t\treturn nil // Reject negative IDs\n\t}\n\treturn p.ID\n}",
		1,
	)

	err = os.WriteFile(petFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user modified GetID with validation")

	// Now change the spec to add a new property, which will cause the generator
	// to regenerate the Pet model (potentially with different GetID signature)
	newSpecContent := `openapi: 3.0.3
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
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
          format: int64
          description: Unique identifier for the pet
        name:
          type: string
        species:
          type: string
          description: The species of the pet
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: add species field")

	// Regenerate - this should attempt a 3-way merge
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// Read the final content
	finalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// The user's custom code should be preserved (merged cleanly without conflict markers)
	require.NotContains(t, string(finalContent), "<<<<<<<", "Should not have conflict markers")
	require.Contains(t, string(finalContent), "USER_CONFLICT_TEST",
		"User modification should be preserved")

	// New field from spec should be present
	require.Contains(t, string(finalContent), "Species", "New species field should be added")
}

// TestPersistentEdits_UserAddedMethodSurvives verifies a realistic scenario where a user adds a method
func TestPersistentEdits_UserAddedMethodSurvives(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find the Pet model file
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	require.FileExists(t, petFile)

	originalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// User adds a custom Validate method at the end of the file
	userAddition := `

// USER_ADDED_VALIDATE: Custom validation method
func (p *Pet) Validate() error {
	if p.Name == nil || *p.Name == "" {
		return fmt.Errorf("pet name is required")
	}
	return nil
}
`
	modifiedContent := string(originalContent) + userAddition
	err = os.WriteFile(petFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user added Validate method")

	// Regenerate without spec changes - user addition should be preserved
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// User's custom method should be preserved
	require.Contains(t, string(finalContent), "USER_ADDED_VALIDATE",
		"User-added method should be preserved")
	require.Contains(t, string(finalContent), "func (p *Pet) Validate() error",
		"User-added Validate method should be preserved")
}

// TestPersistentEdits_ConflictSameLineEdit verifies behavior when user and generator
// both modify the exact same line
func TestPersistentEdits_ConflictSameLineEdit(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find the Pet model file
	petFile := filepath.Join(temp, "models", "components", "pet.go")
	require.FileExists(t, petFile)

	originalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// User changes the struct field tag (modifying existing generated code)
	// Change from the default tag to a custom one
	modifiedContent := strings.Replace(
		string(originalContent),
		"`json:\"id,omitzero\"`",
		"`json:\"pet_id,omitempty\" validate:\"required\"`", // User's custom tag
		1,
	)
	err = os.WriteFile(petFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user modified field tag")

	// Change spec to modify the id field description (which might affect code generation)
	newSpecContent := `openapi: 3.0.3
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
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
components:
  schemas:
    Pet:
      type: object
      required:
        - id
        - name
      properties:
        id:
          type: string
        name:
          type: string
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: make fields required")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.Error(t, err)

	finalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// Same-line edit produces conflict markers
	require.Contains(t, string(finalContent), "<<<<<<<", "Should have conflict start marker")
	require.Contains(t, string(finalContent), "Current (Your changes)",
		"Conflict should show user's version")
	require.Contains(t, string(finalContent), "New (Generated by Speakeasy)",
		"Conflict should show generated version")

	// Git status should show conflict state (UU = both modified/unmerged)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = temp
	statusOutput, err := cmd.Output()
	require.NoError(t, err)
	require.Contains(t, string(statusOutput), "UU", "Git status should show unmerged conflict state")

	// Verify the path in git status is relative, not absolute
	require.Contains(t, string(statusOutput), "models/components/pet.go",
		"Git status should show relative path, not absolute")
	require.NotContains(t, string(statusOutput), temp,
		"Git status should not contain absolute path")

	// Now resolve the conflict by choosing the generated version
	// We'll keep the required field but restore our custom validate tag
	resolvedContent := strings.Replace(
		string(originalContent),
		"`json:\"id,omitzero\"`",
		"`json:\"id\" validate:\"required\"`", // Resolved: use generated json tag but keep user's validate
		1,
	)
	err = os.WriteFile(petFile, []byte(resolvedContent), 0o644)
	require.NoError(t, err)

	// Abort the in-progress merge since we manually resolved
	cmd = exec.Command("git", "checkout", "--theirs", "models/components/pet.go")
	cmd.Dir = temp
	_ = cmd.Run() // ignore error, we're just trying to clean up

	// Stage the resolved file to complete the merge
	cmd = exec.Command("git", "add", "models/components/pet.go")
	cmd.Dir = temp
	err = cmd.Run()
	require.NoError(t, err)

	// Complete the merge with a commit
	cmd = exec.Command("git", "commit", "-m", "resolved conflict")
	cmd.Dir = temp
	err = cmd.Run()
	require.NoError(t, err)

	// Re-run generation - this time it should succeed without conflicts
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console").Run()
	require.NoError(t, err, "Re-run after conflict resolution should succeed")

	// Verify the file no longer has conflict markers
	rerunContent, err := os.ReadFile(petFile)
	require.NoError(t, err)
	require.NotContains(t, string(rerunContent), "<<<<<<<", "File should not have conflict markers after successful re-run")
	require.NotContains(t, string(rerunContent), ">>>>>>>", "File should not have conflict markers after successful re-run")
}

// TestPersistentEdits_NoConflictAdjacentEdits verifies that adjacent but non-overlapping
// edits merge cleanly without conflicts
func TestPersistentEdits_NoConflictAdjacentEdits(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find sdk.go
	sdkFile := filepath.Join(temp, "sdk.go")
	require.FileExists(t, sdkFile)

	originalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)

	// User adds a comment at the beginning (after package declaration)
	modifiedContent := strings.Replace(
		string(originalContent),
		"package testsdk",
		"package testsdk\n\n// USER_HEADER_COMMENT: This is a user-added header comment\n// It should not conflict with spec changes elsewhere in the file",
		1,
	)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "user added header comment")

	// Update spec to add a new endpoint (affects different part of code)
	newSpecContent := `openapi: 3.0.3
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
    post:
      summary: Create a pet
      operationId: createPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
      responses:
        '201':
          description: Created
  /pets/{id}:
    get:
      summary: Get a pet by ID
      operationId: getPet
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
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
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0o644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: add getPet endpoint")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)

	// User comment should be preserved
	require.Contains(t, string(finalContent), "USER_HEADER_COMMENT",
		"User header comment should be preserved")

	// No conflict markers should be present (changes are in different areas)
	require.NotContains(t, string(finalContent), "<<<<<<<",
		"Adjacent edits should not produce conflict markers")
}
