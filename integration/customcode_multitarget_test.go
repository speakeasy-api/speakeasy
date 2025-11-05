package integration_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestMultiTargetCustomCode(t *testing.T) {
	t.Parallel()

	// Build the speakeasy binary once for all subtests (using separate binary name)
	speakeasyBinary := buildSpeakeasyBinaryOnce(t, "speakeasy-customcode-multitarget-test-binary")

	t.Run("BasicWorkflowMultiTarget", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeBasicWorkflow(t, speakeasyBinary)
	})

	t.Run("AllTargetsModified", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeAllTargetsModified(t, speakeasyBinary)
	})

	t.Run("IncrementalCustomCodeToOneTarget", func(t *testing.T) {
		t.Parallel()
		testMultiTargetIncrementalCustomCode(t, speakeasyBinary)
	})

	t.Run("ConflictResolutionAcceptOurs", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeConflictResolutionAcceptOurs(t, speakeasyBinary)
	})

	t.Run("ConflictResolutionAcceptOursMultipleResolutions", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeConflictMultipleResultionsAcceptOurs(t, speakeasyBinary)
	})

	t.Run("ConflictResolutionAcceptTheirs", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeConflictResolutionAcceptTheirs(t, speakeasyBinary)
	})

	t.Run("ConflictResolutionMultipleResolutionsAcceptTheirs", func(t *testing.T) {
		t.Parallel()
		testMultiTargetCustomCodeConflictMultipleResultionsAcceptTheirs(t, speakeasyBinary)
	})

}

// testMultiTargetCustomCodeBasicWorkflow tests basic custom code registration and reapplication
// in a multi-target scenario (go, typescript)
func testMultiTargetCustomCodeBasicWorkflow(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Path to go target file
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")

	// Step 1: Modify only the go target file
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")

	// Step 2: Register custom code
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch file was created only for go target
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist at %s", goPatchFile)

	// Step 4: Verify patch file was NOT created for typescript
	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.True(t, os.IsNotExist(err), "TypeScript patch file should not exist")

	// Step 5: Regenerate all targets
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 6: Verify custom code is present in go target
	verifyCustomCodePresent(t, goFilePath, "// custom code in go target")

	// Step 7: Verify typescript file doesn't have the custom code
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")
	if _, err := os.Stat(tsFilePath); err == nil {
		tsContent, err := os.ReadFile(tsFilePath)
		require.NoError(t, err, "Failed to read typescript file")
		require.NotContains(t, string(tsContent), "custom code in go target", "TypeScript file should not contain go custom code")
	}
}

// testMultiTargetCustomCodeAllTargetsModified tests custom code registration and reapplication
// when all targets (go, typescript) are modified
func testMultiTargetCustomCodeAllTargetsModified(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Modify all target files with target-specific custom code
	// Modify comment lines that are safe to change
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* The name that needs to be", "// custom code in typescript target")

	// Step 2: Register custom code
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for all targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Regenerate all targets
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 5: Verify each target has its own custom code
	goContent, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file")
	require.Contains(t, string(goContent), "custom code in go target", "Go file should contain go custom code")

	tsContent, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file")
	require.Contains(t, string(tsContent), "custom code in typescript target", "TypeScript file should contain typescript custom code")

	// Step 6: Verify no cross-contamination between targets
	require.NotContains(t, string(goContent), "custom code in typescript target", "Go file should not contain typescript custom code")
	require.NotContains(t, string(tsContent), "custom code in go target", "TypeScript file should not contain go custom code")
}

// setupMultiTargetSDKGeneration sets up a test directory with multi-target SDK generation
// and git initialization in the root
func setupMultiTargetSDKGeneration(t *testing.T, speakeasyBinary, inputDoc string) string {
	t.Helper()

	temp := setupCustomCodeTestDir(t)

	// Create workflow file with multiple targets
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

	// Setup two targets: go, typescript
	goOutput := "go"
	tsOutput := "typescript"

	workflowFile.Targets["go-target"] = workflow.Target{
		Target: "go",
		Source: "first-source",
		Output: &goOutput,
	}

	workflowFile.Targets["typescript-target"] = workflow.Target{
		Target: "typescript",
		Source: "first-source",
		Output: &tsOutput,
	}

	if isLocalFileReference(inputDoc) {
		err := copyFile("resources/customcodespec.yaml", fmt.Sprintf("%s/%s", temp, inputDoc))
		require.NoError(t, err)
	}

	err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Run speakeasy run command to generate all targets
	runCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	runCmd.Dir = temp
	runOutput, runErr := runCmd.CombinedOutput()
	require.NoError(t, runErr, "speakeasy run should succeed: %s", string(runOutput))

	// Verify both target directories were generated
	goDirInfo, err := os.Stat(filepath.Join(temp, "go"))
	require.NoError(t, err, "Go directory should exist")
	require.True(t, goDirInfo.IsDir(), "Go should be a directory")

	tsDirInfo, err := os.Stat(filepath.Join(temp, "typescript"))
	require.NoError(t, err, "TypeScript directory should exist")
	require.True(t, tsDirInfo.IsDir(), "TypeScript should be a directory")

	// Initialize git repository in the ROOT directory (not per target)
	initGitRepo(t, temp)

	// Commit all generated files with "clean generation" message
	gitCommit(t, temp, "clean generation")

	// Verify the commit was created with the correct message
	verifyGitCommit(t, temp, "clean generation")

	return temp
}

// testMultiTargetIncrementalCustomCode tests adding custom code to all targets,
// then adding more custom code to only one target (go) and verifying all custom code is preserved
func testMultiTargetIncrementalCustomCode(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Add initial custom code to all targets
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// initial custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* The name that needs to be", "// initial custom code in typescript target")

	// Step 2: Register custom code for all targets
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "first customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for all targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Regenerate all targets
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 5: Verify initial custom code is present in all targets
	goContent, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file")
	require.Contains(t, string(goContent), "initial custom code in go target", "Go file should contain initial custom code")

	tsContent, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file")
	require.Contains(t, string(tsContent), "initial custom code in typescript target", "TypeScript file should contain initial custom code")

	// Commit the regenerated files
	gitCommit(t, temp, "regeneration with initial custom code")

	// Step 6: Add MORE custom code to go target only (on a different line)
	modifyLineInFileByPrefix(t, goFilePath, "// successful operation", "// additional custom code in go target")

	// Step 7: Register the new custom code (should update go patch only)
	customCodeCmd2 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd2.Dir = temp
	customCodeOutput2, customCodeErr2 := customCodeCmd2.CombinedOutput()
	require.NoError(t, customCodeErr2, "second customcode command should succeed: %s", string(customCodeOutput2))

	// Step 8: Regenerate all targets again
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 9: Verify go target has BOTH initial and additional custom code
	goContent, err = os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file")
	require.Contains(t, string(goContent), "initial custom code in go target", "Go file should still contain initial custom code")
	require.Contains(t, string(goContent), "additional custom code in go target", "Go file should contain additional custom code")

	// Step 10: Verify typescript still has its original custom code (unchanged)
	tsContent, err = os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file")
	require.Contains(t, string(tsContent), "initial custom code in typescript target", "TypeScript file should still contain its custom code")
	require.NotContains(t, string(tsContent), "additional custom code", "TypeScript file should not contain additional go custom code")
}

// testMultiTargetCustomCodeConflictResolutionAcceptOurs tests conflict resolution in one target
// while preserving custom code in other targets when accepting custom code changes (ours)
func testMultiTargetCustomCodeConflictResolutionAcceptOurs(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Add custom code to ALL targets
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* @deprecated This namespace", "// custom code in typescript target")

	// Step 2: Register custom code for all targets
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for both targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Modify the spec to cause conflict in GO target only (line 477 affects GetUserByName)
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFileByPrefix(t, specPath, "        description: 'The name that needs to be fetched", "        description: 'spec change'")

	// Step 5: Run speakeasy run - should detect conflict in GO target only
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Step 6: Verify conflict markers present in GO file only
	goContentAfterConflict, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after conflict")
	require.Contains(t, string(goContentAfterConflict), "<<<<<<<", "Go file should contain conflict markers")

	// TypeScript file should NOT have conflict markers
	tsContentAfterConflict, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after conflict")
	require.NotContains(t, string(tsContentAfterConflict), "<<<<<<<", "TypeScript file should not contain conflict markers")
	require.Contains(t, string(tsContentAfterConflict), "custom code in typescript target", "TypeScript file should still have its custom code")

	// Step 7: Resolve the go conflict by accepting spec changes (ours)
	checkoutCmd := exec.Command("git", "checkout", "--ours", goFilePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --ours should succeed: %s", string(checkoutOutput))

	// Step 8: Verify conflict markers are gone in go file
	goContentAfterCheckout, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after checkout")
	require.NotContains(t, string(goContentAfterCheckout), "<<<<<<<", "Go file should not contain conflict markers after checkout")

	// Step 9: Stage the resolved go file
	gitAddCmd := exec.Command("git", "add", goFilePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Step 10: Run customcode command to register the resolution
	customCodeCmd2 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd2.Dir = temp
	customCodeOutput2, customCodeErr2 := customCodeCmd2.CombinedOutput()
	require.NoError(t, customCodeErr2, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput2))

	// Step 11: Verify patch files status
	// Go patch file should be empty or removed
	goPatchContent, err := os.ReadFile(goPatchFile)
	if err == nil {
		require.Empty(t, goPatchContent, "Go patch file should be empty after accepting ours")
	}

	// TypeScript patch file should still exist with its content
	tsPatchContent, err := os.ReadFile(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should still exist")
	require.NotEmpty(t, tsPatchContent, "TypeScript patch file should not be empty")
	require.Contains(t, string(tsPatchContent), "custom code in typescript target", "TypeScript patch should contain typescript custom code")

	// Step 12: Verify gen.lock files
	// Go's gen.lock should NOT contain customCodeCommitHash
	goGenLockPath := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent, err := os.ReadFile(goGenLockPath)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.NotContains(t, string(goGenLockContent), "customCodeCommitHash", "Go gen.lock should not contain customCodeCommitHash after accepting ours")

	// TypeScript's gen.lock should still contain customCodeCommitHash
	tsGenLockPath := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent, err := os.ReadFile(tsGenLockPath)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.Contains(t, string(tsGenLockContent), "customCodeCommitHash", "TypeScript gen.lock should still contain customCodeCommitHash")

	// // Step 13: Run regeneration again
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 14: Verify final state
	// Go file should contain spec change, should NOT contain custom code
	goContentFinal, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after final regeneration")
	require.Contains(t, string(goContentFinal), "spec change", "Go file should contain spec change")
	require.NotContains(t, string(goContentFinal), "custom code in go target", "Go file should not contain custom code after accepting ours")

	// TypeScript file should still contain its custom code
	tsContentFinal, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after final regeneration")
	require.Contains(t, string(tsContentFinal), "custom code in typescript target", "TypeScript file should still contain its custom code")
}


// testMultiTargetCustomCodeConflictMultipleResultionsAcceptOurs tests conflict resolution in two targets
// sequentially.
func testMultiTargetCustomCodeConflictMultipleResultionsAcceptOurs(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Add custom code to ALL targets
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* The name that needs to be", "// custom code in typescript target")

	// Step 2: Register custom code for all targets
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for both targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Modify the spec to cause conflict in GO target only (line 477 affects GetUserByName)
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFileByPrefix(t, specPath, "        description: 'The name that needs to be fetched", "        description: 'spec change'")

// ------First Round
	// Step 5: Run speakeasy run - should detect conflict in GO target only
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Step 6: Verify conflict markers present in GO file only
	goContentAfterConflict, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after conflict")
	require.Contains(t, string(goContentAfterConflict), "<<<<<<<", "Go file should contain conflict markers")

	// TypeScript file should NOT have conflict markers
	tsContentAfterConflict, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after conflict")
	require.NotContains(t, string(tsContentAfterConflict), "<<<<<<<", "TypeScript file should not contain conflict markers")
	require.Contains(t, string(tsContentAfterConflict), "custom code in typescript target", "TypeScript file should still have its custom code")

	// Step 7: Resolve the go conflict by accepting spec changes (ours)
	checkoutCmd := exec.Command("git", "checkout", "--ours", goFilePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --ours should succeed: %s", string(checkoutOutput))

	// Step 8: Verify conflict markers are gone in go file
	goContentAfterCheckout, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after checkout")
	require.NotContains(t, string(goContentAfterCheckout), "<<<<<<<", "Go file should not contain conflict markers after checkout")

	// Step 9: Stage the resolved go file
	gitAddCmd := exec.Command("git", "add", goFilePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Step 10: Run customcode command to register the resolution
	customCodeCmd2 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd2.Dir = temp
	customCodeOutput2, customCodeErr2 := customCodeCmd2.CombinedOutput()
	require.NoError(t, customCodeErr2, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput2))

	// Step 11: Verify patch files status
	// Go patch file should be empty or removed
	goPatchContent, err := os.ReadFile(goPatchFile)
	if err == nil {
		require.Empty(t, goPatchContent, "Go patch file should be empty after accepting ours")
	}

	// TypeScript patch file should still exist with its content
	tsPatchContent, err := os.ReadFile(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should still exist")
	require.NotEmpty(t, tsPatchContent, "TypeScript patch file should not be empty")
	require.Contains(t, string(tsPatchContent), "custom code in typescript target", "TypeScript patch should contain typescript custom code")

	// Step 12: Verify gen.lock files
	// Go's gen.lock should NOT contain customCodeCommitHash
	goGenLockPath := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent, err := os.ReadFile(goGenLockPath)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.NotContains(t, string(goGenLockContent), "customCodeCommitHash", "Go gen.lock should not contain customCodeCommitHash after accepting ours")

	// TypeScript's gen.lock should still contain customCodeCommitHash
	tsGenLockPath := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent, err := os.ReadFile(tsGenLockPath)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.Contains(t, string(tsGenLockContent), "customCodeCommitHash", "TypeScript gen.lock should still contain customCodeCommitHash")

// ------Second Round

	// Step 13: Run regeneration again - should detect conflict in TS target.
	regenCmd2 := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd2.Dir = temp
	regenOutput2, regenErr2 := regenCmd2.CombinedOutput()
	require.Error(t, regenErr2, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput2))
	require.Contains(t, string(regenOutput2), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput2), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")


	// Step 14: Verify conflict markers present in TS file only
	tsContentAfterConflict2, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read ts file after conflict")
	require.Contains(t, string(tsContentAfterConflict2), "<<<<<<<", "TS file should contain conflict markers")

	// Go file should NOT have conflict markers
	goContentAfterConflict2, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read typescript file after conflict")
	require.NotContains(t, string(goContentAfterConflict2), "<<<<<<<", "GO file should not contain conflict markers")

	// Step 15: Resolve the ts conflict by accepting spec changes (ours)
	checkoutCmd2 := exec.Command("git", "checkout", "--ours", tsFilePath)
	checkoutCmd2.Dir = temp
	checkoutOutput2, checkoutErr2 := checkoutCmd2.CombinedOutput()
	require.NoError(t, checkoutErr2, "git checkout --ours should succeed: %s", string(checkoutOutput2))

	// Step 16: Verify conflict markers are gone in ts file
	tsContentAfterCheckout2, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read ts file after checkout")
	require.NotContains(t, string(tsContentAfterCheckout2), "<<<<<<<", "TS file should not contain conflict markers after checkout")

	// Step 17: Stage the resolved ts file
	gitAddCmd2 := exec.Command("git", "add", tsFilePath)
	gitAddCmd2.Dir = temp
	gitAddOutput2, gitAddErr2 := gitAddCmd2.CombinedOutput()
	require.NoError(t, gitAddErr2, "git add should succeed: %s", string(gitAddOutput2))

	// Step 18: Run customcode command to register the resolution
	customCodeCmd3 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd3.Dir = temp
	customCodeOutput3, customCodeErr3 := customCodeCmd3.CombinedOutput()
	require.NoError(t, customCodeErr3, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput3))

	// Step 19: Verify patch files status
	// TS patch file should be empty or removed
	tsPatchContent2, err := os.ReadFile(tsPatchFile)
	if err == nil {
		require.Empty(t, tsPatchContent2, "TS patch file should be empty after accepting ours")
	}

	// Step 20: Verify gen.lock files
	// Go's gen.lock should NOT contain customCodeCommitHash
	goGenLockPath2 := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent2, err := os.ReadFile(goGenLockPath2)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.NotContains(t, string(goGenLockContent2), "customCodeCommitHash", "Go gen.lock should not contain customCodeCommitHash after accepting ours")

	// TypeScript's gen.lock should NOT contain customCodeCommitHash
	tsGenLockPath2 := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent2, err := os.ReadFile(tsGenLockPath2)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.NotContains(t, string(tsGenLockContent2), "customCodeCommitHash", "TS gen.lock should not contain customCodeCommitHash after accepting ours")
// -----------------

	// Step 21: Verify final state
	// Go file should contain spec change, should NOT contain custom code
	goContentFinal, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after final regeneration")
	require.Contains(t, string(goContentFinal), "spec change", "Go file should contain spec change")
	require.NotContains(t, string(goContentFinal), "custom code in go target", "Go file should not contain custom code after accepting ours")

	// TypeScript file should still contain its custom code
	tsContentFinal, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after final regeneration")
	require.Contains(t, string(tsContentFinal), "spec change", "Go file should contain spec change")
	require.NotContains(t, string(tsContentFinal), "custom code in typescript target", "TypeScript file should still contain its custom code")
}

// testMultiTargetCustomCodeConflictResolutionAcceptTheirs tests conflict resolution in one target
// while preserving custom code in other targets when accepting custom code changes (theirs)
func testMultiTargetCustomCodeConflictResolutionAcceptTheirs(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Add custom code to ALL targets
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* @deprecated This namespace", "// custom code in typescript target")

	// Step 2: Register custom code for all targets
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for both targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Modify the spec to cause conflict in GO target only (line 477 affects GetUserByName)
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFileByPrefix(t, specPath, "        description: 'The name that needs to be fetched", "        description: 'spec change'")

	// Step 5: Run speakeasy run - should detect conflict in GO target only
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Step 6: Verify conflict markers present in GO file only
	goContentAfterConflict, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after conflict")
	require.Contains(t, string(goContentAfterConflict), "<<<<<<<", "Go file should contain conflict markers")

	// TypeScript file should NOT have conflict markers
	tsContentAfterConflict, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after conflict")
	require.NotContains(t, string(tsContentAfterConflict), "<<<<<<<", "TypeScript file should not contain conflict markers")
	require.Contains(t, string(tsContentAfterConflict), "custom code in typescript target", "TypeScript file should still have its custom code")

	// Step 7: Resolve the go conflict by accepting custom code changes (theirs)
	checkoutCmd := exec.Command("git", "checkout", "--theirs", goFilePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --theirs should succeed: %s", string(checkoutOutput))

	// Step 8: Verify conflict markers are gone in go file
	goContentAfterCheckout, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after checkout")
	require.NotContains(t, string(goContentAfterCheckout), "<<<<<<<", "Go file should not contain conflict markers after checkout")

	// Step 9: Stage the resolved go file
	gitAddCmd := exec.Command("git", "add", goFilePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Step 10: Run customcode command to register the resolution
	customCodeCmd2 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd2.Dir = temp
	customCodeOutput2, customCodeErr2 := customCodeCmd2.CombinedOutput()
	require.NoError(t, customCodeErr2, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput2))

	// Step 11: Verify patch files status
	// Go patch file should still exist with content since we accepted theirs (custom code)
	goPatchContent, err := os.ReadFile(goPatchFile)
	require.NoError(t, err, "Go patch file should still exist")
	require.NotEmpty(t, goPatchContent, "Go patch file should not be empty after accepting theirs")
	require.Contains(t, string(goPatchContent), "custom code in go target", "Go patch should contain go custom code")

	// TypeScript patch file should still exist with its content
	tsPatchContent, err := os.ReadFile(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should still exist")
	require.NotEmpty(t, tsPatchContent, "TypeScript patch file should not be empty")
	require.Contains(t, string(tsPatchContent), "custom code in typescript target", "TypeScript patch should contain typescript custom code")

	// Step 12: Verify gen.lock files
	// Go's gen.lock should still contain customCodeCommitHash since we kept custom code
	goGenLockPath := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent, err := os.ReadFile(goGenLockPath)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.Contains(t, string(goGenLockContent), "customCodeCommitHash", "Go gen.lock should contain customCodeCommitHash after accepting theirs")

	// TypeScript's gen.lock should still contain customCodeCommitHash
	tsGenLockPath := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent, err := os.ReadFile(tsGenLockPath)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.Contains(t, string(tsGenLockContent), "customCodeCommitHash", "TypeScript gen.lock should still contain customCodeCommitHash")

	// Step 13: Run regeneration again
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 14: Verify final state
	// Go file should contain custom code (since we accepted theirs), and should NOT contain spec change
	goContentFinal, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after final regeneration")
	require.NotContains(t, string(goContentFinal), "spec change", "Go file should not contain spec change after accepting theirs")
	require.Contains(t, string(goContentFinal), "custom code in go target", "Go file should contain custom code after accepting theirs")

	// TypeScript file should still contain its custom code
	tsContentFinal, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after final regeneration")
	require.Contains(t, string(tsContentFinal), "custom code in typescript target", "TypeScript file should still contain its custom code")
}

// testMultiTargetCustomCodeConflictMultipleResultionsAcceptTheirsFromOurs tests conflict resolution in two targets
// sequentially, accepting custom code changes (theirs) instead of spec changes.
func testMultiTargetCustomCodeConflictMultipleResultionsAcceptTheirs(t *testing.T, speakeasyBinary string) {
	temp := setupMultiTargetSDKGeneration(t, speakeasyBinary, "customcodespec.yaml")

	// Paths to all target files
	goFilePath := filepath.Join(temp, "go", "models", "operations", "getuserbyname.go")
	tsFilePath := filepath.Join(temp, "typescript", "src", "models", "operations", "getuserbyname.ts")

	// Step 1: Add custom code to ALL targets
	modifyLineInFileByPrefix(t, goFilePath, "// The name that needs to be", "\t// custom code in go target")
	modifyLineInFileByPrefix(t, tsFilePath, "* The name that needs to be", "// custom code in typescript target")

	// Step 2: Register custom code for all targets
	customCodeCmd := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd.Dir = temp
	customCodeOutput, customCodeErr := customCodeCmd.CombinedOutput()
	require.NoError(t, customCodeErr, "customcode command should succeed: %s", string(customCodeOutput))

	// Step 3: Verify patch files were created for both targets
	goPatchFile := filepath.Join(temp, "go", ".speakeasy", "patches", "custom-code.diff")
	_, err := os.Stat(goPatchFile)
	require.NoError(t, err, "Go patch file should exist")

	tsPatchFile := filepath.Join(temp, "typescript", ".speakeasy", "patches", "custom-code.diff")
	_, err = os.Stat(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should exist")

	// Step 4: Modify the spec to cause conflict in GO target only (line 477 affects GetUserByName)
	specPath := filepath.Join(temp, "customcodespec.yaml")
	modifyLineInFileByPrefix(t, specPath, "        description: 'The name that needs to be fetched", "        description: 'spec change'")

// ------First Round
	// Step 5: Run speakeasy run - should detect conflict in GO target only
	regenCmd := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd.Dir = temp
	regenOutput, regenErr := regenCmd.CombinedOutput()
	require.Error(t, regenErr, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput))
	require.Contains(t, string(regenOutput), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Step 6: Verify conflict markers present in GO file only
	goContentAfterConflict, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after conflict")
	require.Contains(t, string(goContentAfterConflict), "<<<<<<<", "Go file should contain conflict markers")

	// TypeScript file should NOT have conflict markers
	tsContentAfterConflict, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after conflict")
	require.NotContains(t, string(tsContentAfterConflict), "<<<<<<<", "TypeScript file should not contain conflict markers")
	require.Contains(t, string(tsContentAfterConflict), "custom code in typescript target", "TypeScript file should still have its custom code")

	// Step 7: Resolve the go conflict by accepting custom code changes (theirs)
	checkoutCmd := exec.Command("git", "checkout", "--theirs", goFilePath)
	checkoutCmd.Dir = temp
	checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
	require.NoError(t, checkoutErr, "git checkout --theirs should succeed: %s", string(checkoutOutput))

	// Step 8: Verify conflict markers are gone in go file
	goContentAfterCheckout, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after checkout")
	require.NotContains(t, string(goContentAfterCheckout), "<<<<<<<", "Go file should not contain conflict markers after checkout")

	// Step 9: Stage the resolved go file
	gitAddCmd := exec.Command("git", "add", goFilePath)
	gitAddCmd.Dir = temp
	gitAddOutput, gitAddErr := gitAddCmd.CombinedOutput()
	require.NoError(t, gitAddErr, "git add should succeed: %s", string(gitAddOutput))

	// Step 10: Run customcode command to register the resolution
	customCodeCmd2 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd2.Dir = temp
	customCodeOutput2, customCodeErr2 := customCodeCmd2.CombinedOutput()
	require.NoError(t, customCodeErr2, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput2))

	// Step 11: Verify patch files status
	// Go patch file should still exist with content since we accepted theirs (custom code)
	goPatchContent, err := os.ReadFile(goPatchFile)
	require.NoError(t, err, "Go patch file should still exist")
	require.NotEmpty(t, goPatchContent, "Go patch file should not be empty after accepting theirs")
	require.Contains(t, string(goPatchContent), "custom code in go target", "Go patch should contain go custom code")

	// TypeScript patch file should still exist with its content
	tsPatchContent, err := os.ReadFile(tsPatchFile)
	require.NoError(t, err, "TypeScript patch file should still exist")
	require.NotEmpty(t, tsPatchContent, "TypeScript patch file should not be empty")
	require.Contains(t, string(tsPatchContent), "custom code in typescript target", "TypeScript patch should contain typescript custom code")

	// Step 12: Verify gen.lock files
	// Go's gen.lock should still contain customCodeCommitHash since we kept custom code
	goGenLockPath := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent, err := os.ReadFile(goGenLockPath)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.Contains(t, string(goGenLockContent), "customCodeCommitHash", "Go gen.lock should contain customCodeCommitHash after accepting theirs")

	// TypeScript's gen.lock should still contain customCodeCommitHash
	tsGenLockPath := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent, err := os.ReadFile(tsGenLockPath)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.Contains(t, string(tsGenLockContent), "customCodeCommitHash", "TypeScript gen.lock should still contain customCodeCommitHash")

// ------Second Round

	// Step 13: Run regeneration again - should detect conflict in TS target.
	regenCmd2 := exec.Command(speakeasyBinary, "run", "-t", "all", "--pinned", "--skip-compile")
	regenCmd2.Dir = temp
	regenOutput2, regenErr2 := regenCmd2.CombinedOutput()
	require.Error(t, regenErr2, "speakeasy run should exit with error after detecting conflicts: %s", string(regenOutput2))
	require.Contains(t, string(regenOutput2), "CUSTOM CODE CONFLICTS DETECTED", "Output should show conflict detection banner")
	require.Contains(t, string(regenOutput2), "Entering automatic conflict resolution mode", "Output should indicate automatic resolution mode")

	// Step 14: Verify conflict markers present in TS file only
	tsContentAfterConflict2, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read ts file after conflict")
	require.Contains(t, string(tsContentAfterConflict2), "<<<<<<<", "TS file should contain conflict markers")

	// Go file should NOT have conflict markers
	goContentAfterConflict2, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after conflict")
	require.NotContains(t, string(goContentAfterConflict2), "<<<<<<<", "GO file should not contain conflict markers")

	// Step 15: Resolve the ts conflict by accepting custom code changes (theirs)
	checkoutCmd2 := exec.Command("git", "checkout", "--theirs", tsFilePath)
	checkoutCmd2.Dir = temp
	checkoutOutput2, checkoutErr2 := checkoutCmd2.CombinedOutput()
	require.NoError(t, checkoutErr2, "git checkout --theirs should succeed: %s", string(checkoutOutput2))

	// Step 16: Verify conflict markers are gone in ts file
	tsContentAfterCheckout2, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read ts file after checkout")
	require.NotContains(t, string(tsContentAfterCheckout2), "<<<<<<<", "TS file should not contain conflict markers after checkout")

	// Step 17: Stage the resolved ts file
	gitAddCmd2 := exec.Command("git", "add", tsFilePath)
	gitAddCmd2.Dir = temp
	gitAddOutput2, gitAddErr2 := gitAddCmd2.CombinedOutput()
	require.NoError(t, gitAddErr2, "git add should succeed: %s", string(gitAddOutput2))

	// Step 18: Run customcode command to register the resolution
	customCodeCmd3 := exec.Command(speakeasyBinary, "customcode", "--output", "console")
	customCodeCmd3.Dir = temp
	customCodeOutput3, customCodeErr3 := customCodeCmd3.CombinedOutput()
	require.NoError(t, customCodeErr3, "customcode command should succeed after conflict resolution: %s", string(customCodeOutput3))

	// Step 19: Verify patch files status
	// TS patch file should still exist with content since we accepted theirs (custom code)
	tsPatchContent2, err := os.ReadFile(tsPatchFile)
	require.NoError(t, err, "TS patch file should still exist")
	require.NotEmpty(t, tsPatchContent2, "TS patch file should not be empty after accepting theirs")
	require.Contains(t, string(tsPatchContent2), "custom code in typescript target", "TS patch should contain typescript custom code")

	// Step 20: Verify gen.lock files
	// Go's gen.lock should still contain customCodeCommitHash since we kept custom code
	goGenLockPath2 := filepath.Join(temp, "go", ".speakeasy", "gen.lock")
	goGenLockContent2, err := os.ReadFile(goGenLockPath2)
	require.NoError(t, err, "Failed to read go gen.lock")
	require.Contains(t, string(goGenLockContent2), "customCodeCommitHash", "Go gen.lock should contain customCodeCommitHash after accepting theirs")

	// TypeScript's gen.lock should still contain customCodeCommitHash since we kept custom code
	tsGenLockPath2 := filepath.Join(temp, "typescript", ".speakeasy", "gen.lock")
	tsGenLockContent2, err := os.ReadFile(tsGenLockPath2)
	require.NoError(t, err, "Failed to read typescript gen.lock")
	require.Contains(t, string(tsGenLockContent2), "customCodeCommitHash", "TS gen.lock should contain customCodeCommitHash after accepting theirs")
// -----------------

	// Step 21: Run final regeneration to ensure everything works
	runRegeneration(t, speakeasyBinary, temp, true)

	// Step 22: Verify final state
	// Go file should contain custom code (since we accepted theirs), and should NOT contain spec change
	goContentFinal, err := os.ReadFile(goFilePath)
	require.NoError(t, err, "Failed to read go file after final regeneration")
	require.NotContains(t, string(goContentFinal), "spec change", "Go file should not contain spec change after accepting theirs")
	require.Contains(t, string(goContentFinal), "custom code in go target", "Go file should contain custom code after accepting theirs")

	// TypeScript file should contain custom code (since we accepted theirs), and should NOT contain spec change
	tsContentFinal, err := os.ReadFile(tsFilePath)
	require.NoError(t, err, "Failed to read typescript file after final regeneration")
	require.NotContains(t, string(tsContentFinal), "spec change", "TypeScript file should not contain spec change after accepting theirs")
	require.Contains(t, string(tsContentFinal), "custom code in typescript target", "TypeScript file should contain custom code after accepting theirs")
}