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
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"go.uber.org/zap"
)

// RegisterCustomCode registers custom code changes by capturing them as patches in gen.lock
func RegisterCustomCode(ctx context.Context, workflow *run.Workflow, resolve bool, runGenerate func() error) error {
	wf, outDir, err := utils.GetWorkflowAndDir()

	logger := log.From(ctx).With(zap.String("method", "RegisterCustomCode"))

	// Record the current git hash at the very beginning for error recovery
	originalHash, err := getCurrentGitHash()
	if err != nil {
		return fmt.Errorf("failed to get current git hash: %w", err)
	}
	logger.Info("Recorded original git hash for error recovery", zap.String("hash", originalHash.String()))

	// Step 1: Verify main is up to date with origin/main
	if err := verifyMainUpToDate(ctx); err != nil {
		return fmt.Errorf("In order to register your custom code, your local branch must be up to date with origin/main: %w", err)
	}

	// Step 2: Check changeset doesn't include .speakeasy directory changes
	if err := checkNoSpeakeasyChanges(ctx); err != nil {
		return fmt.Errorf("Registering custom code in the .speakeasy directory is not supported: %w", err)
	}

	// Step 3: Check if workflow.yaml references local openapi spec and validate no spec changes
	if err := checkNoLocalSpecChanges(ctx, wf); err != nil {
		return fmt.Errorf("Registering custom code in your openapi spec and related files is not supported: %w", err)
	}

	// Step 4: Capture patchset with git diff for custom code changes
	customCodeDiff, err := captureCustomCodeDiff()
	if err != nil {
		return fmt.Errorf("failed to capture custom code diff: %w", err)
	}

	// If no custom code changes detected, return early
	if customCodeDiff == "" && resolve == false{
		return fmt.Errorf("No custom code changes detected, nothing to register")
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
		if err := applyNewPatch(customCodeDiff); err != nil {
			removeCleanGenerationCommit(ctx, originalHash)
			return fmt.Errorf("conflicts detected when applying new patch.  Please resolve any conflicts, and run `customcode` again.")
		}
	}

	// Step 10: Capture the full combined diff (existing patch + new changes)
	fullCustomCodeDiff, err := captureCustomCodeDiff()
	if err != nil {
		return fmt.Errorf("failed to capture full custom code diff: %w", err)
	}

	// Step 10.5: Compile SDK to verify custom code changes
	target := "all"
	if workflow != nil && workflow.Target != "" {
		target = workflow.Target
	}
	logger.Info("Compiling SDK to verify custom code changes...")
	if err := compileAndLintSDK(ctx, target, outDir); err != nil {
		removeCleanGenerationCommit(ctx, originalHash)
		return fmt.Errorf("custom code changes failed compilation or linting.  Please resolve any compilation/linting errors and run `customcode` again.")
	}

	// Step 11: Update gen.lock with full combined patch
	if err := updateGenLockWithPatch(outDir, fullCustomCodeDiff); err != nil {
		return fmt.Errorf("failed to update gen.lock: %w", err)
	}

	// Step 12: Commit just gen.lock with new patch
	if err := commitGenLock(); err != nil {
		return fmt.Errorf("failed to commit gen.lock: %w", err)
	}

	logger.Info("Successfully registered custom code changes.  Code changes will be applied on top of your code after generation.")
	return nil
}

// ShowCustomCodePatch displays the custom code patch stored in the gen.lock file
func ShowCustomCodePatch(ctx context.Context) error {
	logger := log.From(ctx).With(zap.String("method", "RegisterCustomCode"))

	_, outDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(outDir)
	customCodePatch, exists := cfg.LockFile.Management.AdditionalProperties["customCodePatch"]
	if !exists {
		logger.Warn("No existing custom code patch found")
		return nil
	}

	patchStr, ok := customCodePatch.(string)
	if !ok || patchStr == "" {
		logger.Warn("No existing custom code patch found")
		return nil
	}

	logger.Info("Found custom code patch:")
	logger.Info("----------------------")
	logger.Info(fmt.Sprintf("%s\n", patchStr))

	return nil
}

// Git validation helpers
func verifyMainUpToDate(ctx context.Context) error {
	logger := log.From(ctx)
	logger.Info("Verifying main branch is up to date with origin/main")

	// Fetch origin/main
	/** GO GIT
		err = repo.Fetch(&git.FetchOptions{
		// Optional: configure authentication if needed
		// Auth: &http.BasicAuth{Username: "user", Password: "password"},
	})
	*/
	cmd := exec.Command("git", "fetch", "origin", "main")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch origin/main: %w\nOutput: %s", err, string(output))
	}

	// Check if main is up to date with origin/main
	cmd = exec.Command("git", "rev-list", "--count", "main..origin/main")
	/**
	No go-git support
	*/
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

	/**
	* Can be done with GO GIT, but it's not so obvious
	    head, err := repo.Head()
    if err != nil {
        // Handle error
    }
    commit, err := repo.CommitObject(head.Hash())
    if err != nil {
        // Handle error
    }
    tree, err := commit.Tree()
    if err != nil {
        // Handle error
    }
	patch, err := tree1.Diff(tree2)
    if err != nil {
        // Handle error
    }

    var buf bytes.Buffer
    encoder := diff.NewUnifiedEncoder(&buf)
    err = encoder.Encode(patch)
    if err != nil {
        // Handle error
    }
    fmt.Println(buf.String()) // Prints the unified diff
	*/
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


func commitGenLock() error {
	// Add only the gen.lock file
	/** GO GIT
	w, err := repo.Worktree()
	_, err = w.Add(".speakeasy/gen.lock")
	w.Commit("Register custom code changes", &git.CommitOptions{
		Author: &object.Signature{
			Name: "speakeasybot",
			Email: "..."
		}}})
	*/
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

// compileSDK compiles the SDK to verify custom code changes don't break compilation
func compileAndLintSDK(ctx context.Context, target, outDir string) error {
	// Create generator instance
	g, err := generate.New()
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	// If target is "all", detect each target language from the SDK config
	if target == "all" {
		cfg, err := config.Load(outDir)
		if err != nil {
			return fmt.Errorf("failed to load config to detect language: %w", err)
		}

		// Get the first (and usually only) language from the config
		for lang := range cfg.Config.Languages {
			fmt.Println("Language: " + lang)
			// Call the public Compile method
			if err := g.Compile(ctx, lang, outDir); err != nil {
				return err
			}
			if err := g.Lint(ctx, lang, outDir); err != nil {
				return err
			}
		}
	} else {
		if err := g.Compile(ctx, target, outDir); err != nil {
				return err
		}
		if err := g.Lint(ctx, target, outDir); err != nil {
			return err
		}
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

// removeCleanGenerationCommit removes the clean generation commit by:
// 1. stash local changes
// 2. reset --hard to the original git hash
// 3. stash pop those local changes
func removeCleanGenerationCommit(ctx context.Context, originalHash plumbing.Hash) error {
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
