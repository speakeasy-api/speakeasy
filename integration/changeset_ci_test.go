package integration_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/stretchr/testify/require"
)

func TestChangesets_CIGenerate_CreatesChangesetWithoutLockfileOrVersionChurn(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "baseline generation should succeed")
	gitCommitAll(t, temp, "baseline released snapshot")

	enableChangesetVersionStrategy(t, temp)
	writeChangesetSpec(t, temp)

	beforeCfg, err := config.Load(temp)
	require.NoError(t, err)

	err = executeCIWithEnv(t, temp, nil, "generate", "--mode", "test", "--github-access-token", "test-token", "--working-directory", ".", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.NoError(t, err, "ci generate should succeed in changeset mode")

	changesets := mustGlob(t, filepath.Join(temp, ".speakeasy", "changesets", "*.yaml"))
	require.Len(t, changesets, 1, "expected a single changeset file to be written")

	afterCfg, err := config.Load(temp)
	require.NoError(t, err)

	beforeManagement := beforeCfg.LockFile.Management
	afterManagement := afterCfg.LockFile.Management
	beforeManagement.RepoURL = ""
	beforeManagement.InstallationURL = ""
	afterManagement.RepoURL = ""
	afterManagement.InstallationURL = ""

	require.Equal(t, beforeManagement, afterManagement, "ci generate should not rewrite released management state in changeset mode")
	require.Equal(t, beforeCfg.LockFile.PersistentEdits, afterCfg.LockFile.PersistentEdits, "persistent edit lineage state should stay frozen in gen.lock")
	require.Equal(t, beforeCfg.LockFile.TrackedFiles, afterCfg.LockFile.TrackedFiles, "tracked files should stay frozen in gen.lock")

	genYAML, err := os.ReadFile(filepath.Join(temp, "gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(genYAML), "versionStrategy: changeset")
	require.Contains(t, string(genYAML), "version: 1.0.0", "changeset mode should not rewrite the SDK version in gen.yaml")
}

func TestChangesets_CIGenerate_PreservesExplicitMovedFile(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "baseline generation should succeed")
	gitCommitAll(t, temp, "baseline released snapshot")

	enableChangesetVersionStrategy(t, temp)

	originalPath := filepath.Join(temp, "sdk.go")
	movedPath := filepath.Join(temp, "custom", "sdk.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(movedPath), 0o755))
	require.NoError(t, os.Rename(originalPath, movedPath))

	movedContent, err := os.ReadFile(movedPath)
	require.NoError(t, err)
	updatedContent := strings.Replace(string(movedContent), "package testsdk", "package testsdk\n\n// USER_CUSTOM_COMMENT: kept after move", 1)
	require.NoError(t, os.WriteFile(movedPath, []byte(updatedContent), 0o644))

	err = execute(t, temp, "patches", "move", "--file", "sdk.go", "--to", "custom/sdk.go").Run()
	require.NoError(t, err, "recording the explicit move should succeed")

	writeChangesetSpec(t, temp)

	err = executeCIWithEnv(t, temp, nil, "generate", "--mode", "test", "--github-access-token", "test-token", "--working-directory", ".", "--skip-compile", "--skip-testing", "--skip-release").Run()
	require.NoError(t, err, "ci generate should succeed with an explicitly moved file")

	finalContent, err := os.ReadFile(movedPath)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "USER_CUSTOM_COMMENT: kept after move", "custom content should be preserved in the moved file")

	_, err = os.Stat(originalPath)
	require.True(t, os.IsNotExist(err), "the original generated path should stay absent once the move is recorded")

	changesets := mustGlob(t, filepath.Join(temp, ".speakeasy", "changesets", "*.yaml"))
	require.NotEmpty(t, changesets, "changeset mode should emit a changeset during ci generate")
}

func TestChangesets_PatchFiles_ReleaseCollapsesCustomizationsForLaterReplay(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "baseline generation should succeed")
	gitCommitAll(t, temp, "baseline released snapshot")

	enableChangesetVersionStrategyWithPatchFiles(t, temp, true)
	writeChangesetSpec(t, temp)

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "changeset generation should succeed")

	sdkPath := filepath.Join(temp, "sdk.go")
	sdkContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	customized := strings.Replace(string(sdkContent), "package testsdk", "package testsdk\n\n// CHANGESET_PATCH_FILES: preserve me", 1)
	require.NoError(t, os.WriteFile(sdkPath, []byte(customized), 0o644))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console", "--auto-yes").Run()
	require.NoError(t, err, "rerun should capture the custom edit into the branch changeset")

	changesets := mustGlob(t, filepath.Join(temp, ".speakeasy", "changesets", "*.yaml"))
	require.NotEmpty(t, changesets, "expected a visible changeset before release collapse")

	loadedChangesets, err := changeset.LoadAll(temp)
	require.NoError(t, err)
	require.NotEmpty(t, loadedChangesets)
	sdkEntry, ok := loadedChangesets[0].CustomFiles["sdk.go"]
	require.True(t, ok, "expected branch changeset to capture sdk.go")
	require.Contains(t, sdkEntry.ClaimID, "patch:")
	require.NotEmpty(t, sdkEntry.Patch)
	require.NotEmpty(t, sdkEntry.LineageGitObject, "patchFiles changesets should retain lineage fallback")
	require.NotEmpty(t, sdkEntry.LineageRef, "patchFiles changesets should retain lineage fallback")

	err = executeCIWithEnv(
		t,
		temp,
		map[string]string{"INPUT_MODE": "test"},
		"changeset-release",
		"--github-access-token", "test-token",
		"--working-directory", ".",
		"--skip-compile",
		"--skip-testing",
	).Run()
	require.NoError(t, err, "changeset release should succeed in test mode")

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	patchData, err := os.ReadFile(patchPath)
	require.NoError(t, err)
	require.Contains(t, string(patchData), "CHANGESET_PATCH_FILES: preserve me")

	scrubPersistentRefs(t, temp)

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console").Run()
	require.NoError(t, err, "ordinary run should replay released patch files without persistent refs")

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "CHANGESET_PATCH_FILES: preserve me")

	loadedChangesets, err = changeset.LoadAll(temp)
	require.NoError(t, err)
	for _, cs := range loadedChangesets {
		_, exists := cs.CustomFiles["sdk.go"]
		require.False(t, exists, "released patch baseline should not be rediscovered as fresh custom lineage for sdk.go")
	}
}

func TestChangesets_RunRequiresCaptureBeforeUpdatingBranchClaim(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "baseline generation should succeed")
	gitCommitAll(t, temp, "baseline released snapshot")

	enableChangesetVersionStrategy(t, temp)
	writeChangesetSpec(t, temp)

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "changeset generation should succeed")

	sdkPath := filepath.Join(temp, "sdk.go")
	sdkContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	customized := strings.Replace(string(sdkContent), "package testsdk", "package testsdk\n\n// CHANGESET_CAPTURE_REQUIRED: preserve me", 1)
	require.NoError(t, os.WriteFile(sdkPath, []byte(customized), 0o644))

	output, err := executeOutputWithEnv(
		t,
		temp,
		map[string]string{"SPEAKEASY_DISABLE_TELEMETRY": "true"},
		"run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console",
	)
	require.Error(t, err)
	require.Contains(t, output, "generated SDK files contain unmanaged edits")
	require.Contains(t, output, "speakeasy run --auto-yes")
}

func TestChangesets_RunAutoCapturesDirtyGeneratedFilesIntoBranchChangeset(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "baseline generation should succeed")
	gitCommitAll(t, temp, "baseline released snapshot")

	enableChangesetVersionStrategy(t, temp)
	writeChangesetSpec(t, temp)

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err, "changeset generation should succeed")

	sdkPath := filepath.Join(temp, "sdk.go")
	sdkContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	customized := strings.Replace(string(sdkContent), "package testsdk", "package testsdk\n\n// CHANGESET_AUTO_CAPTURE: preserve me", 1)
	require.NoError(t, os.WriteFile(sdkPath, []byte(customized), 0o644))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console", "--auto-yes").Run()
	require.NoError(t, err, "run should capture dirty changes into the branch changeset before generating")

	loadedChangesets, err := changeset.LoadAll(temp)
	require.NoError(t, err)
	require.NotEmpty(t, loadedChangesets)
	entry, ok := loadedChangesets[0].CustomFiles["sdk.go"]
	require.True(t, ok, "expected branch changeset to capture sdk.go")
	require.Contains(t, entry.Patch, "CHANGESET_AUTO_CAPTURE: preserve me")
	require.Contains(t, entry.ClaimID, "patch:")

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "CHANGESET_AUTO_CAPTURE: preserve me")
}

func enableChangesetVersionStrategy(t *testing.T, dir string) {
	t.Helper()

	enableChangesetVersionStrategyWithPatchFiles(t, dir, false)
}

func enableChangesetVersionStrategyWithPatchFiles(t *testing.T, dir string, patchFiles bool) {
	t.Helper()

	genYAMLContent := `configVersion: 2.0.0
generation:
  sdkClassName: SDK
  maintainOpenAPIOrder: true
  usageSnippets:
    optionalPropertyRendering: withExample
  persistentEdits:
    enabled: true
  versionStrategy: changeset
go:
  version: 1.0.0
  packageName: testsdk
`
	if patchFiles {
		genYAMLContent = strings.Replace(genYAMLContent, "  persistentEdits:\n    enabled: true\n", "  persistentEdits:\n    enabled: true\n    patchFiles: true\n", 1)
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen.yaml"), []byte(genYAMLContent), 0o644))
}

func writeChangesetSpec(t *testing.T, dir string) {
	t.Helper()

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
        email:
          type: string
`

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.yaml"), []byte(specContent), 0o644))
}

func mustGlob(t *testing.T, pattern string) []string {
	t.Helper()

	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	return matches
}
