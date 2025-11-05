package registercustomcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
)

// getTargetOutput returns the target output directory, defaulting to "." if nil
func getTargetOutput(target workflow.Target) string {
	if target.Output == nil {
		return "."
	}
	return *target.Output
}

// getOtherTargetOutputs returns all target output directories except the current one
func getOtherTargetOutputs(wf *workflow.Workflow, currentTargetName string) []string {
	var otherOutputs []string
	for targetName, target := range wf.Targets {
		if targetName != currentTargetName {
			output := getTargetOutput(target)
			if output != "." { // Don't exclude current directory
				otherOutputs = append(otherOutputs, output)
			}
		}
	}
	return otherOutputs
}

// RegisterCustomCode registers custom code changes by capturing them as patches in gen.lock
func RegisterCustomCode(ctx context.Context, runGenerate func(string) error) error {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	logger := log.From(ctx).With(zap.String("method", "RegisterCustomCode"))

	// Check if we're completing conflict resolution
	if isConflictResolutionMode() {
		return completeConflictResolution(ctx, wf)
	}

	// Record the current git hash at the very beginning for error recovery
	originalHash, err := getCurrentGitHash()
	if err != nil {
		return fmt.Errorf("failed to get current git hash: %w", err)
	}
	logger.Info("Recorded original git hash for error recovery", zap.String("hash", originalHash.String()))


	// Step 1: Check changeset doesn't include .speakeasy directory changes
	if err := checkNoSpeakeasyChanges(ctx); err != nil {
		return fmt.Errorf("Registering custom code in the .speakeasy directory is not supported: %w", err)
	}

	// Step 2: Check if workflow.yaml references local openapi spec and validate no spec changes
	if err := checkNoLocalSpecChanges(ctx, wf); err != nil {
		return fmt.Errorf("Registering custom code in your openapi spec and related files is not supported: %w", err)
	}
	targetPatches, err := getPatchesPerTarget(wf)
	if err != nil {
		return err
	}

	// Step 3: Reset working directory to HEAD after capturing patches
	// This removes all user changes (staged and unstaged) so they don't get included in clean generation commit
	logger.Info("Resetting working directory to HEAD before clean generation")
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset working directory: %w\nOutput: %s", err, string(output))
	}

	for _, target := range wf.Targets {
		if err := RevertCustomCodePatch(ctx, target); err != nil {
			return fmt.Errorf("failed to revert custom code patch: %w", err)
		}
	}

	// Step 4: Commit clean generation to preserve metadata
	if err := commitRevertCustomCode(); err != nil {
		return fmt.Errorf("failed to commit clean generation: %w", err)
	}

	for targetName, target := range wf.Targets {
		if targetPatches[targetName] == "" {
			continue
		}
		err = updateCustomPatchAndUpdateGenLock(ctx, wf, originalHash, targetPatches, target, targetName)
		if err != nil {
			return err
		}

	}

	logger.Info("Successfully registered custom code changes.  Code changes will be applied on top of your code after generation.")
	return nil
}

func updateCustomPatchAndUpdateGenLock(ctx context.Context, wf *workflow.Workflow, originalHash plumbing.Hash, targetPatches map[string]string, target workflow.Target, targetName string) error {
	logger := log.From(ctx).With(zap.String("method", "updateCustomPatchAndUpdateGenLock"))
	// Step 7: Apply existing custom code patch from gen.lock
	if err := ApplyCustomCodePatch(ctx, target); err != nil {
		return fmt.Errorf("failed to apply existing patch: %w", err)
	}
	// Step 8: Apply the new custom code diff (with --index to stage changes)
	if err := applyNewPatch(targetPatches[targetName]); err != nil {
		removeReverseCustomCode(ctx, originalHash)
		return fmt.Errorf("conflicts detected when applying new patch.  Please resolve any conflicts, and run `customcode` again.")
	}

	// Check if there are any changes after applying the patch. If no changes, continue the loop
	otherTargetOutputs := getOtherTargetOutputs(wf, targetName)
	hasChanges, err := checkForChangesWithExclusions(getTargetOutput(target), otherTargetOutputs)
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}
	if !hasChanges {
		// Check if there's actually a patch to clean up
		if patchFileExists(getTargetOutput(target)) {
			fmt.Printf("No changes detected for target %s after applying patches, cleaning up patch registration\n", targetName)

			// Clean up: remove patch file and commit hash from gen.lock
			if err := saveCustomCodePatch(getTargetOutput(target), "", ""); err != nil {
				return fmt.Errorf("failed to clean up empty patch: %w", err)
			}

			// Commit the cleanup
			if err := commitCustomCodeRegistration(getTargetOutput(target)); err != nil {
				return fmt.Errorf("failed to commit patch cleanup: %w", err)
			}
		} else {
			fmt.Printf("No changes detected for target %s, skipping\n", targetName)
		}
		return nil
	}

	// Step 10: Capture the full combined diff (existing patch + new changes)
	fullCustomCodeDiff, err := captureCustomCodeDiff(getTargetOutput(target), otherTargetOutputs)
	if err != nil {
		return fmt.Errorf("failed to capture full custom code diff: %w", err)
	}
	targetPatches[targetName] = fullCustomCodeDiff
	logger.Info("Compiling SDK to verify custom code changes...")
	if err := compileAndLintSDK(ctx, target); err != nil {
		removeReverseCustomCode(ctx, originalHash)
		return fmt.Errorf("custom code changes failed compilation or linting.  Please resolve any compilation/linting errors and run `customcode` again.")
	}
	// Step 10.6: Create commit with custom code changes after successful compilation
	customCodeCommitHash, err := commitCustomCodeChanges()
	if err != nil {
		removeReverseCustomCode(ctx, originalHash)
		return fmt.Errorf("failed to commit custom code changes: %w", err)
	}
	logger.Info("Created commit with custom code changes", zap.String("commit_hash", customCodeCommitHash))

	// Step 11: Save custom code patch and update gen.lock with commit hash
	if err := saveCustomCodePatch(getTargetOutput(target), targetPatches[targetName], customCodeCommitHash); err != nil {
		return fmt.Errorf("failed to save custom code patch: %w", err)
	}

	// Step 12: Commit gen.lock and patch file
	if err := commitCustomCodeRegistration(getTargetOutput(target)); err != nil {
		return fmt.Errorf("failed to commit custom code registration: %w", err)
	}
	return nil
}

// ShowCustomCodePatch displays the custom code patch stored in the patch file
func ShowCustomCodePatch(ctx context.Context, target workflow.Target) error {
	logger := log.From(ctx).With(zap.String("method", "ShowCustomCodePatch"))

	outDir := getTargetOutput(target)

	// Read patch from file
	patchStr, err := readPatchFile(outDir)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %w", err)
	}
	if patchStr == "" {
		logger.Warn("No existing custom code patch found")
		return nil
	}

	// Load config to get commit hash
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there's a commit hash associated with this patch
	if customCodeCommitHash, hashExists := cfg.LockFile.Management.AdditionalProperties["customCodeCommitHash"]; hashExists {
		if commitHash, ok := customCodeCommitHash.(string); ok && commitHash != "" {
			logger.Info("Custom Code Commit Hash:", zap.String("hash", commitHash))
		}
	}

	logger.Info("Found custom code patch:")
	logger.Info("----------------------")
	logger.Info(fmt.Sprintf("%s\n", patchStr))

	return nil
}

// ShowLatestCommitHash displays the latest commit hash from gen.lock that contains custom code changes
func ShowLatestCommitHash(ctx context.Context) error {
	logger := log.From(ctx).With(zap.String("method", "ShowLatestCommitHash"))

	_, outDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there's a commit hash stored in gen.lock
	if customCodeCommitHash, hashExists := cfg.LockFile.Management.AdditionalProperties["customCodeCommitHash"]; hashExists {
		if commitHash, ok := customCodeCommitHash.(string); ok && commitHash != "" {
			fmt.Println(commitHash)
			return nil
		}
	}

	logger.Warn("No custom code commit hash found in gen.lock")
	return nil
}

// ResolveCustomCodeConflicts enters conflict resolution mode to help users resolve conflicts
// that occurred during generation when applying custom code patches
func ResolveCustomCodeConflicts(ctx context.Context) error {
	logger := log.From(ctx).With(zap.String("method", "ResolveCustomCodeConflicts"))

	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	hadConflicts := false

	// First pass: identify targets with conflicts and unstage those without conflicts
	// This is necessary because git add . stages everything, including targets that weren't regenerated
	for targetName, target := range wf.Targets {
		outDir := getTargetOutput(target)

		// Check if this target has conflicted files
		checkConflictCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U", "--", outDir)
		conflictCheckOutput, err := checkConflictCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check for conflicts in target %s: %w", targetName, err)
		}

		hasConflicts := strings.TrimSpace(string(conflictCheckOutput)) != ""

		if !hasConflicts {
			// Unstage and restore this target's files to prevent them from being included in conflict resolution
			// This is necessary because the target was never regenerated in this run
			logger.Info(fmt.Sprintf("Target %s has no conflicts, unstaging and restoring its files to HEAD", targetName))

			// First unstage
			unstageCmd := exec.Command("git", "reset", "--", outDir)
			if output, err := unstageCmd.CombinedOutput(); err != nil {
				logger.Warn(fmt.Sprintf("Failed to unstage target %s: %v\nOutput: %s", targetName, err, string(output)))
			}

			// Then restore to HEAD state (removes modifications from working directory)
			checkoutCmd := exec.Command("git", "checkout", "HEAD", "--", outDir)
			if output, err := checkoutCmd.CombinedOutput(); err != nil {
				logger.Warn(fmt.Sprintf("Failed to restore target %s to HEAD: %v\nOutput: %s", targetName, err, string(output)))
			}
		}
	}

	// Second pass: process targets with conflicts
	for targetName, target := range wf.Targets {
		outDir := getTargetOutput(target)

		// Check if patch file exists
		patchStr, err := readPatchFile(outDir)
		if err != nil {
			return fmt.Errorf("failed to read patch file for target %s: %w", targetName, err)
		}
		if patchStr == "" {
			logger.Info(fmt.Sprintf("No custom code patch for target %s, skipping", targetName))
			continue
		}

		// Check if this target actually has conflicted files from the current generation
		// Only targets that were regenerated and have conflicts should be processed
		checkConflictCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U", "--", outDir)
		conflictCheckOutput, err := checkConflictCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check for conflicts in target %s: %w", targetName, err)
		}

		if strings.TrimSpace(string(conflictCheckOutput)) == "" {
			logger.Info(fmt.Sprintf("Target %s has no conflicts (not regenerated in this run), skipping conflict resolution", targetName))
			continue
		}

		logger.Info(fmt.Sprintf("Resolving conflicts for target %s", targetName))

		// Step 1: Undo patch application - extract clean new generation from "ours" side
		cmd := exec.Command("git", "checkout", "--ours", "--", outDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to checkout ours: %w\nOutput: %s", err, string(output))
		}

		// Step 2: Add other changes to worktree (stage the clean generation files)
		if err := stageAllChanges(outDir); err != nil {
			return fmt.Errorf("failed to stage changes for target %s: %w", targetName, err)
		}

		// Step 3: Commit as 'clean generation'
		cmd = exec.Command("git", "commit", "-m", "clean generation (conflict resolution)", "--allow-empty")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to commit clean generation: %w\nOutput: %s", err, string(output))
		}

		// Step 4: Apply old patch (will create conflicts)
		patchFile := filepath.Join(outDir, ".speakeasy", "resolve_patch.patch")
		if err := os.WriteFile(patchFile, []byte(patchStr), 0644); err != nil {
			return fmt.Errorf("failed to write patch file: %w", err)
		}
		defer os.Remove(patchFile)

		cmd = exec.Command("git", "apply", "-3", patchFile)
		_, _ = cmd.CombinedOutput() // Expect failure with conflicts

		// Step 5: Check if conflicts exist
		cmd = exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		conflictOutput, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check for conflicts: %w", err)
		}

		conflictFiles := strings.Split(strings.TrimSpace(string(conflictOutput)), "\n")
		if len(conflictFiles) > 0 && conflictFiles[0] != "" {
			hadConflicts = true
			fmt.Printf("\nConflicts detected in target '%s':\n", targetName)
			for _, file := range conflictFiles {
				fmt.Printf("  - %s\n", file)
			}
		}
	}

	if hadConflicts {
		fmt.Println("\nPlease:")
		fmt.Println("  1. Resolve conflicts in your editor")
		fmt.Println("  2. Stage resolved files: git add <files>")
		fmt.Println("  3. Run: speakeasy customcode")
		fmt.Println("\nThe updated patch will be registered.")
	} else {
		fmt.Println("\nNo conflicts detected. You may proceed with registration.")
	}

	return nil
}

// ensureAllConflictsResolvedAndStaged checks that all git conflicts are resolved and staged
func ensureAllConflictsResolvedAndStaged() error {
	// Check for unmerged paths (conflicts)
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var unresolvedConflicts []string
	var unstagedFiles []string

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filename := line[3:]

		// Check for unmerged paths (conflicts)
		// U = unmerged, AA = both added, UU = both modified, etc.
		if strings.ContainsAny(statusCode, "U") || statusCode == "AA" || statusCode == "DD" {
			unresolvedConflicts = append(unresolvedConflicts, filename)
		}

		// Check for unstaged modifications
		if len(statusCode) >= 2 && statusCode[1] == 'M' {
			unstagedFiles = append(unstagedFiles, filename)
		}
	}

	if len(unresolvedConflicts) > 0 {
		return fmt.Errorf("unresolved git conflicts found in files: %s. Please resolve conflicts and stage the files", strings.Join(unresolvedConflicts, ", "))
	}

	if len(unstagedFiles) > 0 {
		return fmt.Errorf("unstaged changes found in files: %s. Please stage all resolved files with 'git add'", strings.Join(unstagedFiles, ", "))
	}

	// Check for conflict markers in staged changes
	diffCmd := exec.Command("git", "diff", "--cached")
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get staged changes: %w", err)
	}

	diffContent := string(diffOutput)
	conflictMarkers := []string{"<<<<<<<", "=======", ">>>>>>>"}

	for _, marker := range conflictMarkers {
		if strings.Contains(diffContent, marker) {
			return fmt.Errorf("unresolved conflict markers found in staged changes. Please resolve all conflicts (remove %s markers) before continuing", marker)
		}
	}

	return nil
}

// completeConflictResolution completes the conflict resolution process after user has resolved conflicts
func completeConflictResolution(ctx context.Context, wf *workflow.Workflow) error {
	logger := log.From(ctx).With(zap.String("method", "completeConflictResolution"))

	// Ensure all conflicts are resolved and staged before continuing
	if err := ensureAllConflictsResolvedAndStaged(); err != nil {
		return err
	}

	// Record the current git hash at the very beginning for error recovery
	originalHash, err := getCurrentGitHash()
	if err != nil {
		return fmt.Errorf("failed to get current git hash: %w", err)
	}

	logger.Info("Completing conflict resolution registration")

	// First, identify which targets were part of the conflict resolution
	targetsInResolution := make(map[string]bool)
	for targetName, target := range wf.Targets {
		outDir := getTargetOutput(target)
		checkCommitCmd := exec.Command("git", "log", "-1", "--grep=clean generation (conflict resolution)", "--format=%H", "--", outDir)
		commitOutput, err := checkCommitCmd.Output()
		if err == nil && strings.TrimSpace(string(commitOutput)) != "" {
			// Get the most recent commit hash
			headCommitCmd := exec.Command("git", "rev-parse", "HEAD")
			headCommitOutput, headErr := headCommitCmd.Output()
			if headErr == nil {
				commitHash := strings.TrimSpace(string(commitOutput))
				headCommitHash := strings.TrimSpace(string(headCommitOutput))
				
				// Only consider it part of resolution if the commit is the HEAD commit
				if commitHash == headCommitHash {
					targetsInResolution[targetName] = true
					logger.Info(fmt.Sprintf("Target %s was part of conflict resolution", targetName))
				}
			}
		}
	}

	targetPatches, err := getPatchesPerTarget(wf)
	if err != nil {
		return err
	}

	// Only revert patches for targets that were part of conflict resolution
	for targetName, target := range wf.Targets {
		if !targetsInResolution[targetName] {
			logger.Info(fmt.Sprintf("Skipping patch revert for target %s (not part of conflict resolution)", targetName))
			continue
		}

		if err := RevertCustomCodePatch(ctx, target); err != nil {
			// If reverting fails, it might be because the patch was already removed (user accepted ours)
			// Log but continue - we'll handle this in the next phase
			logger.Warn(fmt.Sprintf("Could not revert patch for target %s (may already be reverted): %v", targetName, err))
		}
	}

	for targetName, target := range wf.Targets {
		if targetPatches[targetName] == "" {
			// Check if this target was part of the conflict resolution using the map we built earlier
			if !targetsInResolution[targetName] {
				// This target was not part of conflict resolution (wasn't regenerated)
				// Keep its existing patch unchanged
				logger.Info(fmt.Sprintf("Target %s was not part of conflict resolution, preserving existing patch", targetName))
				continue
			}

			// Target was part of conflict resolution but has no new patches
			// Check if there's actually a patch to clean up
			if patchFileExists(getTargetOutput(target)) {
				fmt.Printf("No changes detected for target %s after conflict resolution, cleaning up patch registration\n", targetName)

				// Clean up: remove patch file and commit hash from gen.lock
				if err := saveCustomCodePatch(getTargetOutput(target), "", ""); err != nil {
					return fmt.Errorf("failed to clean up empty patch: %w", err)
				}

				// Commit the cleanup
				if err := commitCustomCodeRegistration(getTargetOutput(target)); err != nil {
					return fmt.Errorf("failed to commit patch cleanup: %w", err)
				}
			} else {
				fmt.Printf("No changes detected for target %s after conflict resolution, skipping\n", targetName)
			}
			continue
		}
		err = updateCustomPatchAndUpdateGenLock(ctx, wf, originalHash, targetPatches, target, targetName)
		if err != nil {
			return err
		}
	}

	fmt.Println("\nSuccessfully registered updated custom code patches.")
	fmt.Println("Your custom code is now compatible with the latest generation.")

	return nil
}

func getPatchesPerTarget(wf *workflow.Workflow) (map[string]string, error) {
       targetPatches := make(map[string]string)
       for targetName, target := range wf.Targets {
               // Step 4: Capture patchset with git diff for custom code changes
               otherTargetOutputs := getOtherTargetOutputs(wf, targetName)
               customCodeDiff, err := captureCustomCodeDiff(getTargetOutput(target), otherTargetOutputs)
               if err != nil {
                       return nil, fmt.Errorf("failed to capture custom code diff: %w", err)
               }
               fmt.Println(fmt.Sprintf("Captured custom code diff for target %v:\n%s", targetName, customCodeDiff))
               // If no custom code changes detected, return early
               if customCodeDiff == "" {
                       fmt.Println(fmt.Sprintf("No custom code changes detected in target %v, nothing to register", targetName))
               }
               targetPatches[targetName] = customCodeDiff
       }
       return targetPatches, nil
}

func checkNoSpeakeasyChanges(ctx context.Context) error {
	logger := log.From(ctx)
	logger.Info("Checking that changeset doesn't include .speakeasy directory changes")

	cmd := exec.Command("git", "diff", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	speakeasyFiles := []string{}

	for _, file := range files {
		if file != "" && strings.Contains(file, ".speakeasy/") {
			speakeasyFiles = append(speakeasyFiles, file)
		}
	}

	if len(speakeasyFiles) > 0 {
		return fmt.Errorf("changeset contains .speakeasy directory changes: %s", strings.Join(speakeasyFiles, ", "))
	}

	logger.Info("No .speakeasy directory changes found in changeset")
	return nil
}

func checkNoLocalSpecChanges(ctx context.Context, workflow *workflow.Workflow) error {
	logger := log.From(ctx)
	logger.Info("Checking if workflow.yaml references local OpenAPI specs and validating no spec changes")

	// Extract local spec paths from workflow
	localSpecPaths := extractLocalSpecPaths(workflow)
	if len(localSpecPaths) == 0 {
		logger.Info("No local OpenAPI specs referenced in workflow.yaml")
		return nil
	}

	logger.Info("Found local OpenAPI spec paths", zap.Strings("paths", localSpecPaths))

	// Check if any of the local spec files have changes
	cmd := exec.Command("git", "diff", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	changedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	conflictingFiles := []string{}

	for _, specPath := range localSpecPaths {
		for _, changedFile := range changedFiles {
			if changedFile == specPath {
				conflictingFiles = append(conflictingFiles, specPath)
			}
		}
	}

	if len(conflictingFiles) > 0 {
		return fmt.Errorf("changeset contains local openapi spec changes: %s", strings.Join(conflictingFiles, ", "))
	}

	logger.Info("No local OpenAPI spec changes found in changeset")
	return nil
}

func extractLocalSpecPaths(wf *workflow.Workflow) []string {
	var paths []string

	// Check sources directly
	for _, source := range wf.Sources {
		for _, input := range source.Inputs {
			if isLocalPath(input.Location) {
				resolvedPath := input.Location.Resolve()
				paths = append(paths, resolvedPath)
			}
		}
	}

	// Check sources referenced by targets
	for _, target := range wf.Targets {
		if source, exists := wf.Sources[target.Source]; exists {
			for _, input := range source.Inputs {
				if isLocalPath(input.Location) {
					resolvedPath := input.Location.Resolve()
					// Avoid duplicates
					if !slices.Contains(paths, resolvedPath) {
						paths = append(paths, resolvedPath)
					}
				}
			}
		}
	}

	return paths
}

func isLocalPath(location workflow.LocationString) bool {
	resolvedPath := location.Resolve()

	// Check if this is a remote URL
	if strings.HasPrefix(resolvedPath, "https://") || strings.HasPrefix(resolvedPath, "http://") {
		return false
	}

	// Check if this is a registry reference
	if strings.Contains(resolvedPath, "registry.speakeasyapi.dev") {
		return false
	}

	// Check if this is a git reference
	if strings.HasPrefix(resolvedPath, "git+") {
		return false
	}

	// Local paths (relative or absolute)
	return strings.HasPrefix(resolvedPath, "./") ||
		strings.HasPrefix(resolvedPath, "../") ||
		strings.HasPrefix(resolvedPath, "/") ||
		(!strings.Contains(resolvedPath, "://") && !strings.Contains(resolvedPath, "@"))
}

// Git operations
func captureCustomCodeDiff(outDir string, excludePaths []string) (string, error) {
	args := []string{"diff", "HEAD", outDir}

	// Filter excludePaths to only include children of outDir
	cleanOutDir := filepath.Clean(outDir)
	for _, excludePath := range excludePaths {
		cleanExcludePath := filepath.Clean(excludePath)

		// Check if excludePath is a child of outDir (or equal to outDir)
		rel, err := filepath.Rel(cleanOutDir, cleanExcludePath)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			args = append(args, ":^"+excludePath)
		}
	}

	cmd := exec.Command("git", args...)
	combinedOutput, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("failed to capture git diff: %w", err)
	}

	return string(combinedOutput), nil
}

func checkForChangesWithExclusions(dir string, excludePaths []string) (bool, error) {
	args := []string{"diff", "--cached", dir}

	// Filter excludePaths to only include children of dir
	cleanDir := filepath.Clean(dir)
	for _, excludePath := range excludePaths {
		cleanExcludePath := filepath.Clean(excludePath)

		// Check if excludePath is a child of dir (or equal to dir)
		rel, err := filepath.Rel(cleanDir, cleanExcludePath)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			args = append(args, ":^"+excludePath)
		}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("failed to check for changes: %w", err)
	}

	// Check if output is empty (no changes) or has content (changes exist)
	return strings.TrimSpace(string(output)) != "", nil
}

func stageAllChanges(dir string) error {
	if dir == "" {
		dir = "."
	}
	// Add all changes
	addCmd := exec.Command("git", "add", dir)
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add changes: %w\nOutput: %s", err, string(output))
	}

	return nil
}


func commitRevertCustomCode() error {
	// Add all changes
	addCmd := exec.Command("git", "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add changes for clean generation commit: %w\nOutput: %s", err, string(output))
	}

	// Commit the clean generation (allow empty if nothing changed)
	cmd := exec.Command("git", "commit", "-m", "reverse apply custom code", "--allow-empty")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func commitCustomCodeChanges() (string, error) {
	// Commit the staged changes (changes should already be staged by --index operations)
	commitMsg := "Apply custom code changes"
	cmd := exec.Command("git", "commit", "-m", commitMsg, "--allow-empty")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to commit custom code changes: %w\nOutput: %s", err, string(output))
	}

	// Get the commit hash
	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	commitHash := strings.TrimSpace(string(hashOutput))
	return commitHash, nil
}

func commitCustomCodeRegistration(outDir string) error {
	// Add gen.lock and patch file
	genLockPath := fmt.Sprintf("%v/.speakeasy/gen.lock", outDir)
	patchPath := getPatchFilePath(outDir)

	// Always add gen.lock
	cmd := exec.Command("git", "add", genLockPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add gen.lock: %w", err)
	}

	// Handle patch file - add if exists, stage deletion if removed
	if patchFileExists(outDir) {
		cmd = exec.Command("git", "add", patchPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add patch file: %w", err)
		}
	} else {
		// Stage deletion using git rm (won't fail if file not tracked)
		cmd = exec.Command("git", "rm", "--ignore-unmatch", patchPath)
		_ = cmd.Run() // Ignore errors - file might not exist in git
	}

	// Commit with a descriptive message
	commitMsg := "Register custom code changes"
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit custom code registration: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func ApplyCustomCodePatch(ctx context.Context, target workflow.Target) error {
	outDir := getTargetOutput(target)

	// Check if patch file exists
	if !patchFileExists(outDir) {
		return nil // No patch to apply
	}

	// Read patch content to verify it's not empty
	patchContent, err := readPatchFile(outDir)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %w", err)
	}
	if patchContent == "" {
		return nil // Empty patch, nothing to apply
	}

	// Apply the patch directly from file with 3-way merge
	patchFile := getPatchFilePath(outDir)
	args := []string{"apply", "--3way", "--index", patchFile}
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply patch: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func RevertCustomCodePatch(ctx context.Context, target workflow.Target) error {
	outDir := getTargetOutput(target)

	// Check if patch file exists
	if !patchFileExists(outDir) {
		return nil // No patch to revert
	}

	// Read patch content to verify it's not empty
	patchContent, err := readPatchFile(outDir)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %w", err)
	}
	if patchContent == "" {
		return nil // Empty patch, nothing to revert
	}

	// Revert the patch directly from file with reverse flag
	patchFile := getPatchFilePath(outDir)
	args := []string{"apply", "--reverse", "--index", patchFile}
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to revert patch: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func applyNewPatch(customCodeDiff string) error {
	if customCodeDiff == "" {
		return nil
	}

	// Create a temporary patch file
	patchFile := ".speakeasy/temp_new_patch.diff"
	if err := os.WriteFile(patchFile, []byte(customCodeDiff), 0644); err != nil {
		return fmt.Errorf("failed to write new patch file: %w", err)
	}
	defer os.Remove(patchFile)

	// Apply the patch with 3-way merge and stage changes
	cmd := exec.Command("git", "apply", "-3", "--index", patchFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply new patch: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Patch file helper functions

// getPatchFilePath returns the standardized path for the custom code patch file
func getPatchFilePath(outDir string) string {
	return filepath.Join(outDir, ".speakeasy", "patches", "custom-code.diff")
}

// ensurePatchesDirectoryExists creates the patches directory if it doesn't exist
func ensurePatchesDirectoryExists(outDir string) error {
	patchesDir := filepath.Join(outDir, ".speakeasy", "patches")
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		return fmt.Errorf("failed to create patches directory: %w", err)
	}
	return nil
}

// writePatchFile writes the patch content to the custom code patch file
func writePatchFile(outDir, patchContent string) error {
	if err := ensurePatchesDirectoryExists(outDir); err != nil {
		return err
	}

	patchPath := getPatchFilePath(outDir)
	if err := os.WriteFile(patchPath, []byte(patchContent), 0644); err != nil {
		return fmt.Errorf("failed to write patch file: %w", err)
	}

	return nil
}

// readPatchFile reads the patch content from the custom code patch file
// Returns empty string if file doesn't exist (not an error)
func readPatchFile(outDir string) (string, error) {
	patchPath := getPatchFilePath(outDir)

	content, err := os.ReadFile(patchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No patch found
		}
		return "", fmt.Errorf("failed to read patch file: %w", err)
	}

	return string(content), nil
}

// deletePatchFile deletes the custom code patch file
func deletePatchFile(outDir string) error {
	patchPath := getPatchFilePath(outDir)
	if err := os.Remove(patchPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete patch file: %w", err)
	}
	return nil
}

// patchFileExists checks if the custom code patch file exists
func patchFileExists(outDir string) bool {
	patchPath := getPatchFilePath(outDir)
	_, err := os.Stat(patchPath)
	return err == nil
}

func saveCustomCodePatch(outDir, patchset, commitHash string) error {
	// Load the current configuration and lock file
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize AdditionalProperties if nil
	if cfg.LockFile.Management.AdditionalProperties == nil {
		cfg.LockFile.Management.AdditionalProperties = make(map[string]any)
	}

	// Write patch to file
	if patchset != "" {
		if err := writePatchFile(outDir, patchset); err != nil {
			return fmt.Errorf("failed to write patch file: %w", err)
		}
		// Store the commit hash in gen.lock
		if commitHash != "" {
			cfg.LockFile.Management.AdditionalProperties["customCodeCommitHash"] = commitHash
		}
	} else {
		// Remove patch file and commit hash if empty
		if err := deletePatchFile(outDir); err != nil {
			return fmt.Errorf("failed to delete patch file: %w", err)
		}
		delete(cfg.LockFile.Management.AdditionalProperties, "customCodeCommitHash")
	}

	// Save the updated gen.lock
	if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
		return fmt.Errorf("failed to save gen.lock: %w", err)
	}

	return nil
}

// compileSDK compiles the SDK to verify custom code changes don't break compilation
func compileAndLintSDK(ctx context.Context, target workflow.Target) error {
	// Create generator instance
	g, err := generate.New()
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	if err := g.Compile(ctx, target.Target, getTargetOutput(target)); err != nil {
		return err
	}
	if err := g.Lint(ctx, target.Target, getTargetOutput(target)); err != nil {
		return err
	}

	return nil
}

// getCurrentGitHash returns the current git commit hash
func getCurrentGitHash() (plumbing.Hash, error) {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to open git repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	return head.Hash(), nil
}

// removeReverseCustomCode removes the reverse custom code commit by:
// 1. stash local changes
// 2. reset --hard to the original git hash
// 3. stash pop those local changes
func removeReverseCustomCode(ctx context.Context, originalHash plumbing.Hash) error {
	logger := log.From(ctx).With(zap.String("method", "removeCleanGenerationCommit"))
	logger.Info("Starting error recovery process", zap.String("target_hash", originalHash.String()))

	// Step 1: Stash local changes using git command
	logger.Info("Stashing local changes")
	stashCmd := exec.Command("git", "stash", "push", "-m", "RegisterCustomCode error recovery stash")
	stashOutput, stashErr := stashCmd.CombinedOutput()
	stashSuccessful := stashErr == nil && !strings.Contains(string(stashOutput), "No local changes to save")

	if stashErr != nil && !strings.Contains(string(stashOutput), "No local changes to save") {
		logger.Warn("Failed to stash changes, continuing with reset", zap.Error(stashErr), zap.String("output", string(stashOutput)))
	} else if stashSuccessful {
		logger.Info("Successfully stashed changes")
	} else {
		logger.Info("No changes to stash")
	}

	// Step 2: Reset --hard to the original git hash using go-git
	logger.Info("Resetting to original git hash", zap.String("hash", originalHash.String()))
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open git repository for recovery: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree for recovery: %w", err)
	}

	err = worktree.Reset(&git.ResetOptions{
		Commit: originalHash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset to original hash %s: %w", originalHash.String(), err)
	}

	// Step 3: Stash pop those local changes (if we successfully stashed)
	if stashSuccessful {
		logger.Info("Popping stashed changes")
		popCmd := exec.Command("git", "stash", "pop")
		if popOutput, popErr := popCmd.CombinedOutput(); popErr != nil {
			logger.Error("Failed to pop stashed changes, but reset was successful", zap.Error(popErr), zap.String("output", string(popOutput)))
			return fmt.Errorf("reset successful but failed to restore stashed changes: %w", popErr)
		}
		logger.Info("Successfully restored stashed changes")
	}

	logger.Info("Error recovery completed successfully")
	return nil
}

// isConflictResolutionMode checks if we're in conflict resolution mode by checking the HEAD commit message
func isConflictResolutionMode() bool {
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	msg := strings.TrimSpace(string(output))
	return msg == "clean generation (conflict resolution)"
}
