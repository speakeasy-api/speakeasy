package integration_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

// TestPersistentEdits_BasicHeaderGeneration verifies that @generated-id headers are added to generated files
func TestPersistentEdits_BasicHeaderGeneration(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Run generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "Initial generation should succeed")

	// Check 3 sample files that should have @generated-id headers
	sampleFiles := []string{
		filepath.Join(temp, "sdk.go"),
		filepath.Join(temp, "models", "components", "pet.go"),
		filepath.Join(temp, "models", "components", "httpmetadata.go"),
	}

	uuidPattern := regexp.MustCompile(`@generated-id:\s+([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`)
	seenUUIDs := make(map[string]string) // uuid -> filename

	for _, file := range sampleFiles {
		require.FileExists(t, file, "Sample file should exist")

		content, err := os.ReadFile(file)
		require.NoError(t, err, "Failed to read file: %s", file)

		match := uuidPattern.FindSubmatch(content)
		require.NotNil(t, match, "File %s should have @generated-id header", file)

		uuid := string(match[1])
		t.Logf("File %s has UUID: %s", filepath.Base(file), uuid)

		// Verify UUID is unique
		if existingFile, exists := seenUUIDs[uuid]; exists {
			t.Errorf("Duplicate UUID %s found in %s and %s", uuid, existingFile, file)
		}
		seenUUIDs[uuid] = file
	}

	require.Len(t, seenUUIDs, 3, "Should have 3 unique UUIDs")
}

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

	// Read original content and extract ID
	originalContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	t.Logf("sdk.go after first generation (first 500 chars):\n%s", string(originalContent[:min(500, len(originalContent))]))

	generatedID := extractGeneratedIDFromContent(originalContent)
	require.NotEmpty(t, generatedID)
	t.Logf("Generated ID: %s", generatedID)

	// Add a user comment
	modifiedContent := strings.Replace(string(originalContent), "package testsdk", "package testsdk\n\n// USER_CUSTOM_COMMENT: This is my custom code", 1)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)
	t.Logf("Modified sdk.go (first 500 chars):\n%s", string(modifiedContent[:min(500, len(modifiedContent))]))

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

	// Verify @generated-id is still present and unchanged
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, generatedID, finalID, "Generated ID should remain the same")
}

// --- Helper functions for persistent edits tests ---

// setupPersistentEditsTestDir creates a test directory with a minimal OpenAPI spec and workflow
// configured for persistent edits
func setupPersistentEditsTestDir(t *testing.T) string {
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
	err := os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create .speakeasy directory
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0755)
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
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Create .genignore to exclude go.mod/go.sum from generation (avoids dependency conflicts in tests)
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

// findGeneratedGoFiles finds all .go files in the temp directory (excluding vendor, .git, etc.)
func findGeneratedGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden directories and vendor
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	return files
}

// extractGeneratedIDFromContent extracts the @generated-id UUID from file content
func extractGeneratedIDFromContent(content []byte) string {
	pattern := regexp.MustCompile(`@generated-id:\s+([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`)
	match := pattern.FindSubmatch(content)
	if len(match) > 1 {
		return string(match[1])
	}
	return ""
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
	goID := extractGeneratedIDFromContent(goContent)
	require.NotEmpty(t, goID, "Go SDK should have @generated-id")

	tsContent, err := os.ReadFile(tsSdkFile)
	require.NoError(t, err)
	tsID := extractGeneratedIDFromContent(tsContent)
	require.NotEmpty(t, tsID, "TypeScript SDK should have @generated-id")

	// Modify both SDKs with user comments
	goModified := strings.Replace(string(goContent), "package gosdk", "package gosdk\n\n// GO_USER_COMMENT: Custom Go code", 1)
	err = os.WriteFile(goSdkFile, []byte(goModified), 0644)
	require.NoError(t, err)

	tsModified := strings.Replace(string(tsContent), "export class SDK extends ClientSDK", "// TS_USER_COMMENT: Custom TypeScript code\nexport class SDK extends ClientSDK", 1)
	err = os.WriteFile(tsSdkFile, []byte(tsModified), 0644)
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

	// Verify IDs are preserved
	goFinalID := extractGeneratedIDFromContent(goFinal)
	require.Equal(t, goID, goFinalID, "Go generated ID should remain the same")

	tsFinalID := extractGeneratedIDFromContent(tsFinal)
	require.Equal(t, tsID, tsFinalID, "TypeScript generated ID should remain the same")
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
	petID := extractGeneratedIDFromContent(petContent)
	require.NotEmpty(t, petID)

	// Add user comment to Pet model
	petModified := strings.Replace(string(petContent), "type Pet struct", "// PET_USER_COMMENT: Custom validation logic here\ntype Pet struct", 1)
	err = os.WriteFile(petFile, []byte(petModified), 0644)
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
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0644)
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

	// Verify ID is preserved
	petFinalID := extractGeneratedIDFromContent(petFinal)
	require.Equal(t, petID, petFinalID, "Generated ID should remain the same after spec change")
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

	originalIDs := make(map[string]string)
	for file, comment := range filesToModify {
		require.FileExists(t, file, "File should exist: %s", file)

		content, err := os.ReadFile(file)
		require.NoError(t, err)

		id := extractGeneratedIDFromContent(content)
		require.NotEmpty(t, id, "File should have @generated-id: %s", file)
		originalIDs[file] = id

		// Add comment after package declaration
		modified := strings.Replace(string(content), "package ", comment+"\npackage ", 1)
		err = os.WriteFile(file, []byte(modified), 0644)
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

		finalID := extractGeneratedIDFromContent(content)
		require.Equal(t, originalIDs[file], finalID, "Generated ID should remain the same in %s", file)
	}
}

// setupMultiTargetPersistentEditsTestDir creates a test directory with multiple targets configured
func setupMultiTargetPersistentEditsTestDir(t *testing.T) string {
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

	// Create output directories
	err = os.MkdirAll(filepath.Join(temp, "go-sdk"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(temp, "ts-sdk"), 0755)
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

	// Create gen.yaml with persistent edits enabled for both targets
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
  packageName: gosdk
typescript:
  version: 1.0.0
  packageName: tssdk
`
	err = os.WriteFile(filepath.Join(temp, "gen.yaml"), []byte(genYamlContent), 0644)
	require.NoError(t, err)

	// Create .genignore
	genignoreContent := `go.mod
go.sum
package.json
package-lock.json
node_modules
`
	err = os.WriteFile(filepath.Join(temp, ".genignore"), []byte(genignoreContent), 0644)
	require.NoError(t, err)

	// Initialize git repo
	gitInit(t, temp)
	gitCommitAll(t, temp, "initial commit")

	return temp
}

func stringPtr(s string) *string {
	return &s
}

// TestPersistentEdits_FileMove verifies that user edits are preserved when a file is moved
func TestPersistentEdits_FileMove(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find the Pet model file
	originalPath := filepath.Join(temp, "models", "components", "pet.go")
	require.FileExists(t, originalPath)

	// Read and get the generated ID
	originalContent, err := os.ReadFile(originalPath)
	require.NoError(t, err)
	originalID := extractGeneratedIDFromContent(originalContent)
	require.NotEmpty(t, originalID, "Pet model should have @generated-id")
	t.Logf("Original Pet ID: %s", originalID)

	// Add a user comment to the file
	modifiedContent := strings.Replace(string(originalContent), "type Pet struct", "// PET_MOVED_FILE: This comment should survive the move\ntype Pet struct", 1)

	// Create a new directory and move the file there
	newDir := filepath.Join(temp, "mymodels")
	err = os.MkdirAll(newDir, 0755)
	require.NoError(t, err)

	newPath := filepath.Join(newDir, "pet.go")
	err = os.WriteFile(newPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Remove the original file
	err = os.Remove(originalPath)
	require.NoError(t, err)

	gitCommitAll(t, temp, "moved pet.go to mymodels/ with user comment")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// The file should be regenerated at the new location (following the UUID)
	require.FileExists(t, newPath, "File should exist at moved path")

	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)

	// UUID tracking should work - ID should be preserved
	newID := extractGeneratedIDFromContent(newContent)
	require.Equal(t, originalID, newID, "Generated ID should be preserved after move")

	// User modification should be preserved
	require.Contains(t, string(newContent), "PET_MOVED_FILE: This comment should survive the move",
		"User modification should be preserved after file move")
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

	// Get the IDs before removal
	petContent, err := os.ReadFile(petFile)
	require.NoError(t, err)
	petID := extractGeneratedIDFromContent(petContent)
	require.NotEmpty(t, petID)

	sdkContent, err := os.ReadFile(sdkFile)
	require.NoError(t, err)
	sdkID := extractGeneratedIDFromContent(sdkContent)
	require.NotEmpty(t, sdkID)

	// Add user modification to sdk.go (to verify it's preserved)
	sdkModified := strings.Replace(string(sdkContent), "package testsdk", "package testsdk\n\n// SDK_PRESERVED: This should survive", 1)
	err = os.WriteFile(sdkFile, []byte(sdkModified), 0644)
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

	sdkFinalID := extractGeneratedIDFromContent(sdkFinal)
	require.Equal(t, sdkID, sdkFinalID, "SDK generated ID should remain the same")

	// pet.go will not be regenerated since the user deleted it
	_, err = os.Stat(petFile)
	require.Error(t, err, "pet.go should not be regenerated after user deletion")
	require.True(t, os.IsNotExist(err), "pet.go should not exist")
}

// TestPersistentEdits_FileRename verifies that renaming a file preserves edits via @generated-id tracking
func TestPersistentEdits_FileRename(t *testing.T) {
	t.Parallel()
	temp := setupPersistentEditsTestDir(t)

	// Initial generation
	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitCommitAll(t, temp, "initial generation")

	// Find an operation file
	originalPath := filepath.Join(temp, "models", "operations", "listpets.go")
	require.FileExists(t, originalPath)

	// Read and get the generated ID
	originalContent, err := os.ReadFile(originalPath)
	require.NoError(t, err)
	originalID := extractGeneratedIDFromContent(originalContent)
	require.NotEmpty(t, originalID, "listpets.go should have @generated-id")
	t.Logf("Original listpets.go ID: %s", originalID)

	// Add a user comment
	modifiedContent := strings.Replace(string(originalContent), "package operations", "package operations\n\n// RENAMED_FILE_COMMENT: User customization here", 1)

	// Rename the file (same directory, different name)
	newPath := filepath.Join(temp, "models", "operations", "list_all_pets.go")
	err = os.WriteFile(newPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Remove the original
	err = os.Remove(originalPath)
	require.NoError(t, err)

	gitCommitAll(t, temp, "renamed listpets.go to list_all_pets.go with user comment")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	// File should exist at new path with preserved content
	require.FileExists(t, newPath, "File should exist at renamed location")

	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)

	newID := extractGeneratedIDFromContent(newContent)
	require.Equal(t, originalID, newID, "Generated ID should be preserved after rename")

	require.Contains(t, string(newContent), "RENAMED_FILE_COMMENT: User customization here",
		"User modification should be preserved after file rename")
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
	originalID := extractGeneratedIDFromContent(originalContent)
	require.NotEmpty(t, originalID)
	t.Logf("Original Pet ID: %s", originalID)

	// User modifies the GetID method to add custom validation
	// This conflicts because the generator will also want to generate GetID
	modifiedContent := strings.Replace(
		string(originalContent),
		"func (p *Pet) GetID() *int64 {\n\tif p == nil {\n\t\treturn nil\n\t}\n\treturn p.ID\n}",
		"func (p *Pet) GetID() *int64 {\n\t// USER_CONFLICT_TEST: Custom validation\n\tif p == nil {\n\t\treturn nil\n\t}\n\tif p.ID != nil && *p.ID < 0 {\n\t\treturn nil // Reject negative IDs\n\t}\n\treturn p.ID\n}",
		1,
	)

	err = os.WriteFile(petFile, []byte(modifiedContent), 0644)
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
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0644)
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

	// ID should be preserved
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, originalID, finalID, "Generated ID should remain the same")
}

// TestPersistentEdits_ConflictUserAddedMethod verifies conflict handling when user adds a method
// that the spec also tries to generate
func TestPersistentEdits_ConflictUserAddedMethod(t *testing.T) {
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
	err = os.WriteFile(petFile, []byte(modifiedContent), 0644)
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
	originalID := extractGeneratedIDFromContent(originalContent)

	// User changes the struct field tag (modifying existing generated code)
	// Change from the default tag to a custom one
	modifiedContent := strings.Replace(
		string(originalContent),
		"`json:\"id,omitzero\"`",
		"`json:\"pet_id,omitempty\" validate:\"required\"`", // User's custom tag
		1,
	)
	err = os.WriteFile(petFile, []byte(modifiedContent), 0644)
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
          type: integer
          format: int64
        name:
          type: string
`
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0644)
	require.NoError(t, err)
	gitCommitAll(t, temp, "spec update: make fields required")

	// Regenerate
	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(petFile)
	require.NoError(t, err)

	// Same-line edit produces conflict markers
	require.Contains(t, string(finalContent), "<<<<<<<", "Should have conflict start marker")
	require.Contains(t, string(finalContent), "Current (Your changes)",
		"Conflict should show user's version")
	require.Contains(t, string(finalContent), "New (Generated)",
		"Conflict should show generated version")

	// Git status should show conflict state (UU = both modified/unmerged)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = temp
	statusOutput, err := cmd.Output()
	require.NoError(t, err)
	require.Contains(t, string(statusOutput), "UU", "Git status should show unmerged conflict state")

	// ID should be preserved regardless
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, originalID, finalID, "Generated ID should be preserved")
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
	originalID := extractGeneratedIDFromContent(originalContent)

	// User adds a comment at the beginning (after package declaration)
	modifiedContent := strings.Replace(
		string(originalContent),
		"package testsdk",
		"package testsdk\n\n// USER_HEADER_COMMENT: This is a user-added header comment\n// It should not conflict with spec changes elsewhere in the file",
		1,
	)
	err = os.WriteFile(sdkFile, []byte(modifiedContent), 0644)
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
	err = os.WriteFile(filepath.Join(temp, "spec.yaml"), []byte(newSpecContent), 0644)
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

	// ID should be preserved
	finalID := extractGeneratedIDFromContent(finalContent)
	require.Equal(t, originalID, finalID, "Generated ID should be preserved")
}
