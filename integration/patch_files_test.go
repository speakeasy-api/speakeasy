package integration_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchFiles_CaptureAndReplayWithoutPersistentRefs(t *testing.T) {
	t.Parallel()

	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	sdkPath := filepath.Join(temp, "sdk.go")
	pristineContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	customizedContent := strings.Replace(string(pristineContent), "package testsdk", "package testsdk\n\n// PATCH_FILES_CAPTURED: replay me", 1)
	require.NoError(t, os.WriteFile(sdkPath, []byte(customizedContent), 0o644))

	err = execute(t, temp, "patches", "capture", "--dir", temp, "--target", "all", "--skip-compile").Run()
	require.NoError(t, err)

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	patchData, err := os.ReadFile(patchPath)
	require.NoError(t, err)
	assert.Contains(t, string(patchData), "PATCH_FILES_CAPTURED: replay me")

	gitForceCommitAll(t, temp, "capture released patch baseline")
	updateSpecWithGetPetOperation(t, temp)
	scrubPersistentRefs(t, temp)
	require.NoFileExists(t, filepath.Join(temp, "models", "operations", "getpet.go"))
	require.NoFileExists(t, filepath.Join(temp, "docs", "models", "operations", "getpetrequest.md"))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	assert.Contains(t, string(finalContent), "PATCH_FILES_CAPTURED: replay me")
	assert.Contains(t, string(finalContent), "GetPet")
}

func TestPatchFiles_DirtyDiskRequiresCaptureBeforeRun(t *testing.T) {
	t.Parallel()

	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	sdkPath := filepath.Join(temp, "sdk.go")
	content, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	content = []byte(strings.Replace(string(content), "package testsdk", "package testsdk\n\n// PATCH_FILES_CAPTURED: baseline", 1))
	require.NoError(t, os.WriteFile(sdkPath, content, 0o644))

	err = execute(t, temp, "patches", "capture", "--dir", temp, "--target", "all", "--skip-compile").Run()
	require.NoError(t, err)

	updatedContent := string(content) + `

func patchFilesDirtyFallback() string {
	return "PATCH_FILES_DIRTY_FALLBACK: keep me"
}
`
	require.NoError(t, os.WriteFile(sdkPath, []byte(updatedContent), 0o644))

	output, err := executeOutputWithEnv(
		t,
		temp,
		map[string]string{"SPEAKEASY_DISABLE_TELEMETRY": "true"},
		"run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console",
	)
	require.Error(t, err)
	assert.Contains(t, output, "generated SDK files contain unmanaged edits")
	assert.Contains(t, output, "speakeasy patches capture")

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	assert.Contains(t, string(finalContent), "PATCH_FILES_CAPTURED: baseline")
	assert.Contains(t, string(finalContent), "PATCH_FILES_DIRTY_FALLBACK: keep me")
}

func TestPatchFiles_RunAutoCapturesDirtyGeneratedFiles(t *testing.T) {
	t.Parallel()

	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	sdkPath := filepath.Join(temp, "sdk.go")
	content, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	updatedContent := string(content) + `

func patchFilesAutoCapture() string {
	return "PATCH_FILES_AUTO_CAPTURE: keep me"
}
`
	require.NoError(t, os.WriteFile(sdkPath, []byte(updatedContent), 0o644))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console", "--auto-yes").Run()
	require.NoError(t, err)

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	patchData, err := os.ReadFile(patchPath)
	require.NoError(t, err)
	assert.Contains(t, string(patchData), "PATCH_FILES_AUTO_CAPTURE: keep me")

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	assert.Contains(t, string(finalContent), "PATCH_FILES_AUTO_CAPTURE: keep me")
}

func TestPatchFiles_ManualGitDiffPatchReplays(t *testing.T) {
	t.Parallel()

	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	sdkPath := filepath.Join(temp, "sdk.go")
	originalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	modifiedContent := strings.Replace(string(originalContent), "package testsdk", "package testsdk\n\n// PATCH_FILES_MANUAL: replay me", 1)
	gitForceCommitAll(t, temp, "generated baseline for manual patch")
	require.NoError(t, os.WriteFile(sdkPath, []byte(modifiedContent), 0o644))

	diffCmd := exec.Command("git", "diff", "--", "sdk.go")
	diffCmd.Dir = temp
	patchData, err := diffCmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		require.Equal(t, 1, exitErr.ExitCode(), "git diff should exit 1 when differences are found")
	}

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	require.NoError(t, os.MkdirAll(filepath.Dir(patchPath), 0o755))
	require.NoError(t, os.WriteFile(patchPath, patchData, 0o644))
	require.NoError(t, os.WriteFile(sdkPath, originalContent, 0o644))

	gitForceCommitAll(t, temp, "commit manual patch baseline")
	updateSpecWithGetPetOperation(t, temp)
	scrubPersistentRefs(t, temp)
	require.NoFileExists(t, filepath.Join(temp, "models", "operations", "getpet.go"))
	require.NoFileExists(t, filepath.Join(temp, "docs", "models", "operations", "getpetrequest.md"))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	assert.Contains(t, string(finalContent), "PATCH_FILES_MANUAL: replay me")
	assert.Contains(t, string(finalContent), "GetPet")
}

func TestPatchFiles_RejectsManualMultiFilePatch(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitForceCommitAll(t, temp, "generated baseline for invalid multi-file patch")

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	require.NoError(t, os.MkdirAll(filepath.Dir(patchPath), 0o755))
	require.NoError(t, os.WriteFile(patchPath, []byte(`diff --git a/sdk.go b/sdk.go
--- a/sdk.go
+++ b/sdk.go
@@ -1,3 +1,4 @@
 package testsdk
 
+// sdk custom
 type SDK struct{}
diff --git a/models/components/pet.go b/models/components/pet.go
--- a/models/components/pet.go
+++ b/models/components/pet.go
@@ -1,3 +1,4 @@
 package components
 
+// pet custom
type Pet struct{}
`), 0o644))

	updateSpecWithGetPetOperation(t, temp)
	scrubPersistentRefs(t, temp)

	output, err := executeOutputWithEnv(
		t,
		temp,
		map[string]string{"SPEAKEASY_DISABLE_TELEMETRY": "true"},
		"run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console",
	)
	require.Error(t, err)
	assert.Contains(t, output, "invalid patch file for sdk.go")
	assert.Contains(t, output, "expected exactly 1 file diff")
	assert.Contains(t, output, "speakeasy patches capture")
}

func TestPatchFiles_RejectsManualRenamePatch(t *testing.T) {
	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)
	gitForceCommitAll(t, temp, "generated baseline for invalid rename patch")

	patchPath := filepath.Join(temp, ".speakeasy", "patches", "sdk.go.patch")
	require.NoError(t, os.MkdirAll(filepath.Dir(patchPath), 0o755))
	require.NoError(t, os.WriteFile(patchPath, []byte(`diff --git a/sdk.go b/custom/sdk.go
similarity index 100%
rename from sdk.go
rename to custom/sdk.go
`), 0o644))

	updateSpecWithGetPetOperation(t, temp)
	scrubPersistentRefs(t, temp)

	output, err := executeOutputWithEnv(
		t,
		temp,
		map[string]string{"SPEAKEASY_DISABLE_TELEMETRY": "true"},
		"run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console",
	)
	require.Error(t, err)
	assert.Contains(t, output, "invalid patch file for sdk.go")
	assert.Contains(t, output, `patch headers target "sdk.go" -> "custom/sdk.go"`)
	assert.Contains(t, output, "speakeasy patches capture")
}

func TestPatchFiles_ReplaysAcrossOpenAPIChangeOnCleanGeneratedCheckout(t *testing.T) {
	t.Parallel()

	temp := setupPersistentEditsTestDir(t)
	enablePatchFilesInGenYAML(t, temp)

	err := execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--output", "console").Run()
	require.NoError(t, err)

	sdkPath := filepath.Join(temp, "sdk.go")
	pristineContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)

	customizedContent := strings.Replace(string(pristineContent), "package testsdk", "package testsdk\n\n// PATCH_FILES_ACROSS_OAS: replay me", 1)
	require.NoError(t, os.WriteFile(sdkPath, []byte(customizedContent), 0o644))

	err = execute(t, temp, "patches", "capture", "--dir", temp, "--target", "all", "--skip-compile").Run()
	require.NoError(t, err)

	gitForceCommitAll(t, temp, "capture released patch baseline")

	specPath := filepath.Join(temp, "spec.yaml")
	specContent, err := os.ReadFile(specPath)
	require.NoError(t, err)
	updatedSpec := strings.Replace(string(specContent), "components:\n  schemas:\n", `  /pets/{id}:
    get:
      summary: Get a pet
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
`, 1)
	require.NoError(t, os.WriteFile(specPath, []byte(updatedSpec), 0o644))

	scrubPersistentRefs(t, temp)
	require.NoFileExists(t, filepath.Join(temp, "models", "operations", "getpet.go"))
	require.NoFileExists(t, filepath.Join(temp, "docs", "models", "operations", "getpetrequest.md"))

	err = execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile", "--skip-versioning", "--output", "console").Run()
	require.NoError(t, err)

	finalContent, err := os.ReadFile(sdkPath)
	require.NoError(t, err)
	assert.Contains(t, string(finalContent), "PATCH_FILES_ACROSS_OAS: replay me")
	assert.Contains(t, string(finalContent), "GetPet")
}

func enablePatchFilesInGenYAML(t *testing.T, dir string) {
	t.Helper()

	genYAMLPath := filepath.Join(dir, "gen.yaml")
	content, err := os.ReadFile(genYAMLPath)
	require.NoError(t, err)

	updated := strings.Replace(string(content), "  persistentEdits:\n    enabled: true\n", "  persistentEdits:\n    enabled: true\n    patchFiles: true\n", 1)
	require.NoError(t, os.WriteFile(genYAMLPath, []byte(updated), 0o644))
}

func updateSpecWithGetPetOperation(t *testing.T, dir string) {
	t.Helper()

	specPath := filepath.Join(dir, "spec.yaml")
	specContent, err := os.ReadFile(specPath)
	require.NoError(t, err)

	updatedSpec := strings.Replace(string(specContent), "components:\n  schemas:\n", `  /pets/{id}:
    get:
      summary: Get a pet
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
`, 1)
	require.NoError(t, os.WriteFile(specPath, []byte(updatedSpec), 0o644))
}

func scrubPersistentRefs(t *testing.T, dir string) {
	t.Helper()

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg.LockFile)

	cfg.LockFile.PersistentEdits = nil
	cfg.LockFile.TrackedFiles = config.NewLockFile().TrackedFiles

	require.NoError(t, config.SaveLockFile(dir, cfg.LockFile))
	_ = os.RemoveAll(filepath.Join(dir, ".git", "refs", "speakeasy"))
}

func executeOutput(t *testing.T, wd string, args ...string) (string, error) {
	t.Helper()

	runner := execute(t, wd, args...).(*subprocessRunner)
	err := runner.Run()
	return runner.out.String(), err
}

func executeOutputWithEnv(t *testing.T, wd string, envOverrides map[string]string, args ...string) (string, error) {
	t.Helper()

	runner := executeWithEnv(t, wd, envOverrides, args...).(*subprocessRunner)
	err := runner.Run()
	return runner.out.String(), err
}

func gitForceCommitAll(t *testing.T, dir string, message string) {
	t.Helper()

	cmd := exec.Command("git", "add", "-A", "-f")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "git add -A -f failed: %s", string(output))

	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = dir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))
}
