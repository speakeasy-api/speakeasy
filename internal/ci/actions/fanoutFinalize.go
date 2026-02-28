package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	cigit "github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	sharedgit "github.com/speakeasy-api/speakeasy/internal/git"
)

type FanoutFinalizeInputs struct {
	BaseBranch         string
	WorkerBranches     string
	TargetBranch       string
	ReportsDir         string
	CleanupPaths       string
	PostGenerateScript string
	CommitMessage      string
	CleanupWorkers     bool
}

func FanoutFinalize(ctx context.Context, inputs FanoutFinalizeInputs) error {
	g, err := initAction()
	if err != nil {
		return err
	}

	baseBranch := strings.TrimSpace(inputs.BaseBranch)
	if baseBranch == "" {
		baseBranch = environment.GetSourceBranch()
	}
	if baseBranch == "" {
		return fmt.Errorf("base branch is required")
	}

	workerBranches := parseListInput(inputs.WorkerBranches)
	if len(workerBranches) == 0 {
		return fmt.Errorf("at least one worker branch is required")
	}

	reportDirInput := strings.TrimSpace(inputs.ReportsDir)
	if reportDirInput == "" {
		reportDirInput = reportsDir
	}
	reportsPath := resolvePathFromWorkingDirectory(reportDirInput)

	cleanupPaths := parseListInput(inputs.CleanupPaths)
	if len(cleanupPaths) == 0 {
		cleanupPaths = []string{reportDirInput, ".speakeasy/logs/changes"}
	}

	repoDir := filepath.Join(g.GetRepoRoot(), environment.GetWorkingDirectory())

	if _, err := runGit(repoDir, "fetch", "origin", baseBranch); err != nil {
		return fmt.Errorf("failed to fetch base branch %s: %w", baseBranch, err)
	}
	if _, err := runGit(repoDir, "checkout", baseBranch); err != nil {
		return fmt.Errorf("failed to checkout base branch %s: %w", baseBranch, err)
	}
	if _, err := runGit(repoDir, "reset", "--hard", "origin/"+baseBranch); err != nil {
		return fmt.Errorf("failed to reset base branch %s: %w", baseBranch, err)
	}

	baseSHA, err := gitHeadSHA(repoDir)
	if err != nil {
		return err
	}

	for _, workerBranch := range workerBranches {
		if _, err := runGit(repoDir, "fetch", "origin", fmt.Sprintf("refs/heads/%s", workerBranch)); err != nil {
			return fmt.Errorf("failed to fetch worker branch %s: %w", workerBranch, err)
		}

		workerCommit, err := gitRevParse(repoDir, "FETCH_HEAD")
		if err != nil {
			return fmt.Errorf("failed to resolve worker branch %s head: %w", workerBranch, err)
		}

		if _, err := runGit(repoDir, "cherry-pick", workerCommit); err != nil {
			return fmt.Errorf("failed to cherry-pick worker commit %s from %s: %w", workerCommit, workerBranch, err)
		}
	}

	output, mergedReport, err := GeneratePRFromReports(reportsPath)
	if err != nil {
		return err
	}

	if strings.TrimSpace(inputs.PostGenerateScript) != "" {
		if err := runPostGenerateScript(g.GetRepoRoot(), resolvePathFromWorkingDirectory(inputs.PostGenerateScript)); err != nil {
			return err
		}
	}

	if _, err := runGit(repoDir, "reset", "--soft", baseSHA); err != nil {
		return fmt.Errorf("failed to start squash from %s: %w", baseSHA, err)
	}

	for _, cleanupPath := range cleanupPaths {
		cleanupPath = strings.TrimSpace(cleanupPath)
		if cleanupPath == "" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(g.GetRepoRoot(), resolvePathFromWorkingDirectory(cleanupPath))); err != nil {
			return fmt.Errorf("failed to remove cleanup path %s: %w", cleanupPath, err)
		}
	}

	if _, err := runGit(repoDir, "add", "-A"); err != nil {
		return fmt.Errorf("failed to stage squashed changes: %w", err)
	}

	status, err := runGit(repoDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to inspect staged changes: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return fmt.Errorf("no staged changes found after fanout finalization")
	}

	commitMessage := strings.TrimSpace(inputs.CommitMessage)
	if commitMessage == "" && mergedReport != nil {
		commitMessage = strings.TrimSpace(mergedReport.GetCommitMarkdownSection())
	}
	if commitMessage == "" {
		commitMessage = "ci: regenerated via fanout workflow"
	}

	if _, err := runGit(repoDir, "commit", "-m", commitMessage); err != nil {
		return fmt.Errorf("failed to create squashed commit: %w", err)
	}

	squashSHA, err := gitHeadSHA(repoDir)
	if err != nil {
		return err
	}

	targetBranch := strings.TrimSpace(inputs.TargetBranch)
	if targetBranch == "" {
		targetBranch, err = resolveTargetBranchName(g, baseBranch)
		if err != nil {
			return err
		}
	}

	if _, err := runGit(repoDir, "checkout", "-B", targetBranch, "origin/"+baseBranch); err != nil {
		return fmt.Errorf("failed to checkout target branch %s: %w", targetBranch, err)
	}
	if _, err := runGit(repoDir, "cherry-pick", squashSHA); err != nil {
		return fmt.Errorf("failed to apply squashed commit %s on %s: %w", squashSHA, targetBranch, err)
	}
	if _, err := runGit(repoDir, "push", "--force", "origin", targetBranch); err != nil {
		return fmt.Errorf("failed to force push %s: %w", targetBranch, err)
	}

	if err := createOrUpdatePRFromGenerated(ctx, targetBranch, output, mergedReport, false); err != nil {
		return err
	}

	if inputs.CleanupWorkers {
		for _, workerBranch := range workerBranches {
			if workerBranch == "" || workerBranch == targetBranch || workerBranch == baseBranch {
				continue
			}
			if err := g.DeleteBranch(workerBranch); err != nil {
				logging.Info("failed to delete worker branch %s: %v", workerBranch, err)
			}
		}
	}

	return nil
}

func resolveTargetBranchName(g *cigit.Git, baseBranch string) (string, error) {
	branchName, _, err := g.FindExistingPR("", environment.ActionRunWorkflow, false)
	if err != nil {
		return "", err
	}
	if branchName != "" {
		return branchName, nil
	}

	sourceBranch := environment.GetSourceBranch()
	if sourceBranch == "" {
		sourceBranch = baseBranch
	}

	timestamp := time.Now().Unix()
	if environment.IsMainBranch(sourceBranch) {
		return fmt.Sprintf("speakeasy-sdk-regen-%d", timestamp), nil
	}

	return fmt.Sprintf("speakeasy-sdk-regen-%s-%d", environment.SanitizeBranchName(sourceBranch), timestamp), nil
}

func runPostGenerateScript(repoRoot, scriptPath string) error {
	resolvedScript := scriptPath
	if !filepath.IsAbs(resolvedScript) {
		resolvedScript = filepath.Join(repoRoot, resolvedScript)
	}
	cmd := exec.Command("bash", resolvedScript)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run post-generation script %s: %w", resolvedScript, err)
	}
	return nil
}

func parseListInput(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n'
	})

	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

func resolvePathFromWorkingDirectory(path string) string {
	clean := filepath.Clean(path)
	workingDirectory := strings.TrimSpace(environment.GetWorkingDirectory())
	if workingDirectory == "" || workingDirectory == "." {
		return clean
	}

	return filepath.Join(workingDirectory, clean)
}

func runGit(repoDir string, args ...string) (string, error) {
	return sharedgit.RunGitCommand(repoDir, args...)
}

func gitRevParse(repoDir, revision string) (string, error) {
	out, err := runGit(repoDir, "rev-parse", revision)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitHeadSHA(repoDir string) (string, error) {
	return gitRevParse(repoDir, "HEAD")
}
