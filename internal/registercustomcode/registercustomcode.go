package registercustomcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"go.uber.org/zap"
)

// RegisterCustomCode registers custom code changes by capturing them as patches in gen.lock
func RegisterCustomCode(ctx context.Context, workflow *run.Workflow, runGenerate func() error) error {
	wf, outDir, err := utils.GetWorkflowAndDir()

	logger := log.From(ctx).With(zap.String("method", "RegisterCustomCode"))

	// Step 1: Verify main is up to date with origin/main
	if err := verifyMainUpToDate(ctx); err != nil {
		return fmt.Errorf("main branch verification failed: %w", err)
	}

	// Step 2: Check changeset doesn't include .speakeasy directory changes
	if err := checkNoSpeakeasyChanges(ctx); err != nil {
		return fmt.Errorf("changeset validation failed: %w", err)
	}

	// Step 3: Check if workflow.yaml references local openapi spec and validate no spec changes
	if err := checkNoLocalSpecChanges(ctx, wf); err != nil {
		return fmt.Errorf("openapi spec validation failed: %w", err)
	}

	// Step 4: Capture patchset with git diff for custom code changes
	customCodeDiff, err := captureCustomCodeDiff()
	if err != nil {
		return fmt.Errorf("failed to capture custom code diff: %w", err)
	}

	// If no custom code changes detected, return early
	if customCodeDiff == "" {
		logger.Info("No custom code changes detected, nothing to register")
		return nil
	}

	// Step 5: Generate clean SDK (without custom code) on main branch
	if err := generateCleanSDK(ctx, workflow, runGenerate); err != nil {
		return fmt.Errorf("failed to generate clean SDK: %w", err)
	}

	// Step 6: Commit clean generation to preserve metadata
	if err := commitCleanGeneration(); err != nil {
		return fmt.Errorf("failed to commit clean generation: %w", err)
	}

	// Step 7: Apply existing custom code patch from gen.lock
	if err := applyCustomCodePatch(outDir); err != nil {
		return fmt.Errorf("failed to apply existing patch: %w", err)
	}

	// Step 8: Stage all changes after applying existing patch
	if err := stageAllChanges(); err != nil {
		return fmt.Errorf("failed to stage changes after applying existing patch: %w", err)
	}

	// Step 9: Apply the new custom code diff
	if customCodeDiff != "" {
		// Emit the new patch before applying it
		if err := emitNewPatch(ctx, customCodeDiff); err != nil {
			logger.Warn("Failed to emit new patch", zap.Error(err))
		}

		if err := applyNewPatch(customCodeDiff); err != nil {
			logger.Warn("Conflicts detected when applying new patch")
			return fmt.Errorf("conflicts detected when applying new patch: %w", err)
		}
	}

	// Step 10: Capture the full combined diff (existing patch + new changes)
	fullCustomCodeDiff, err := captureCustomCodeDiff()
	if err != nil {
		return fmt.Errorf("failed to capture full custom code diff: %w", err)
	}

	// TODO: compile and lint

	// Step 11: Update gen.lock with full combined patch
	if err := updateGenLockWithPatch(outDir, fullCustomCodeDiff); err != nil {
		return fmt.Errorf("failed to update gen.lock: %w", err)
	}

	// Step 12: Commit just gen.lock with new patch
	if err := commitGenLock(); err != nil {
		return fmt.Errorf("failed to commit gen.lock: %w", err)
	}

	// Step 13: Emit/output the full patch for visibility
	if err := emitFullPatch(ctx, fullCustomCodeDiff); err != nil {
		logger.Warn("Failed to emit full patch", zap.Error(err))
	}

	logger.Info("Successfully registered custom code changes")
	return nil
}

// ShowCustomCodePatch displays the custom code patch stored in the gen.lock file
func ShowCustomCodePatch() error {
	_, outDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(outDir)
	customCodePatch, exists := cfg.LockFile.Management.AdditionalProperties["customCodePatch"]
	if !exists {
		fmt.Println("No custom code patch found in gen.lock")
		return nil
	}

	patchStr, ok := customCodePatch.(string)
	if !ok || patchStr == "" {
		fmt.Println("No custom code patch found in gen.lock")
		return nil
	}

	fmt.Println("Found custom code patch:")
	fmt.Println("----------------------")
	fmt.Printf("%s\n", patchStr)

	return nil
}

// Git validation helpers
func verifyMainUpToDate(ctx context.Context) error {
	logger := log.From(ctx)
	logger.Info("Verifying main branch is up to date with origin/main")

	// Fetch origin/main
	cmd := exec.Command("git", "fetch", "origin", "main")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch origin/main: %w\nOutput: %s", err, string(output))
	}

	// Check if main is up to date with origin/main
	cmd = exec.Command("git", "rev-list", "--count", "main..origin/main")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check main status: %w", err)
	}

	count := strings.TrimSpace(string(output))
	if count != "0" {
		return fmt.Errorf("main is not up to date with origin/main (%s commits behind)", count)
	}

	logger.Info("Main branch is up to date with origin/main")
	return nil
}

func checkNoSpeakeasyChanges(ctx context.Context) error {
	logger := log.From(ctx)
	logger.Info("Checking that changeset doesn't include .speakeasy directory changes")

	cmd := exec.Command("git", "diff", "--name-only", "main")
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
	cmd := exec.Command("git", "diff", "--name-only", "main")
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

func generateCleanSDK(ctx context.Context, workflow *run.Workflow, runGenerate func() error) error {
	logger := log.From(ctx)
	err := runGenerate()

	defer func() {
		// we should leave temp directories for debugging if run fails
		if err == nil || env.IsGithubAction() {
			workflow.Cleanup()
		}
	}()

	
	if err != nil {
		return fmt.Errorf("failed to generate SDK: %w", err)
	}

	logger.Info("Clean SDK generation completed successfully")
	return nil
}

// Git operations
func captureCustomCodeDiff() (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture git diff: %w", err)
	}

	return string(output), nil
}

func stageAllChanges() error {
	// Add all changes
	addCmd := exec.Command("git", "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add changes: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func unstageAllChanges() error {
	resetCmd := exec.Command("git", "reset")
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset changes: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func commitCleanGeneration() error {
	// Add all changes
	addCmd := exec.Command("git", "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add changes for clean generation commit: %w\nOutput: %s", err, string(output))
	}

	// Commit the clean generation
	commitCmd := exec.Command("git", "commit", "-m", "clean generation")
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to commit clean generation: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func resetToCleanState(ctx context.Context) error {
	logger := log.From(ctx)
	logger.Info("Resetting to clean state")

	// Reset all changes to get back to a clean state
	cmd := exec.Command("git", "reset", "--hard", "HEAD")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset to clean state: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Successfully reset to clean state")
	return nil
}

func commitGenLock() error {
	// Add only the gen.lock file
	cmd := exec.Command("git", "add", ".speakeasy/gen.lock")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add gen.lock: %w", err)
	}

	// Commit with a descriptive message
	commitMsg := "Register custom code changes"
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit gen.lock: %w", err)
	}

	return nil
}

// Patch management
func applyCustomCodePatch(outDir string) error {
	// Load the current configuration and lock file
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Add and commit changes before applying custom code patch
	if err := stageAllChanges(); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Check if there's a custom code patch in the management section
	if customCodePatch, exists := cfg.LockFile.Management.AdditionalProperties["customCodePatch"]; exists {
		if patchStr, ok := customCodePatch.(string); ok && patchStr != "" {
			// Create a temporary patch file
			patchFile := filepath.Join(outDir, ".speakeasy", "temp_patch.patch")
			if err := os.WriteFile(patchFile, []byte(patchStr), 0644); err != nil {
				return fmt.Errorf("failed to write patch file: %w", err)
			}
			defer os.Remove(patchFile)

			// Apply the patch with 3-way merge
			cmd := exec.Command("git", "apply", "-3", patchFile)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to apply patch: %w\nOutput: %s", err, string(output))
			}
		}
	}

	if err := unstageAllChanges(); err != nil {
		return fmt.Errorf("failed to reset changes: %w", err)
	}

	return nil
}

func applyNewPatch(customCodeDiff string) error {
	if customCodeDiff == "" {
		return nil
	}

	// Create a temporary patch file
	patchFile := ".speakeasy/temp_new_patch.patch"
	if err := os.WriteFile(patchFile, []byte(customCodeDiff), 0644); err != nil {
		return fmt.Errorf("failed to write new patch file: %w", err)
	}
	defer os.Remove(patchFile)

	// Apply the patch with 3-way merge
	cmd := exec.Command("git", "apply", "-3", patchFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply new patch: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func updateGenLockWithPatch(outDir, patchset string) error {
	// Load the current configuration and lock file
	cfg, err := config.Load(outDir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize AdditionalProperties if nil
	if cfg.LockFile.Management.AdditionalProperties == nil {
		cfg.LockFile.Management.AdditionalProperties = make(map[string]any)
	}

	// Store single patch (replaces any existing patch)
	if patchset != "" {
		cfg.LockFile.Management.AdditionalProperties["customCodePatch"] = patchset
	} else {
		// Remove the patch if empty
		delete(cfg.LockFile.Management.AdditionalProperties, "customCodePatch")
	}

	// Save the updated gen.lock
	if err := config.SaveLockFile(outDir, cfg.LockFile); err != nil {
		return fmt.Errorf("failed to save gen.lock: %w", err)
	}

	return nil
}

func emitNewPatch(ctx context.Context, newPatch string) error {
	logger := log.From(ctx)
	logger.Info("Emitting new custom code patch")

	if newPatch == "" {
		fmt.Println("No new custom code changes to apply.")
		return nil
	}

	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Println("NEW CUSTOM CODE PATCH (about to apply)")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println(newPatch)
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("")

	return nil
}

func emitFullPatch(ctx context.Context, fullPatch string) error {
	logger := log.From(ctx)
	logger.Info("Emitting full custom code patch")

	if fullPatch == "" {
		fmt.Println("No custom code changes detected.")
		return nil
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("FULL CUSTOM CODE PATCH")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(fullPatch)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("")

	return nil
}
