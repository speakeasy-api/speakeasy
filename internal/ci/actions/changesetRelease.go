package actions

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-github/v63/github"
	diffParser "github.com/speakeasy-api/git-diff-parser"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/changeset"
	config "github.com/speakeasy-api/sdk-gen-config"
	sdkworkflow "github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	cigit "github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/logging"
	"github.com/speakeasy-api/speakeasy/internal/ci/run"
	"github.com/speakeasy-api/speakeasy/internal/ci/versioninfo"
)

type changesetReleaseTarget struct {
	TargetName string
	OutputDir  string
	Changesets []*changeset.Changeset
}

type changesetReleaseSummary struct {
	TargetName  string
	OutputDir   string
	Version     string
	BumpType    string
	Changesets  []*changeset.Changeset
	Operations  []string
	BranchNames []string
	PatchFiles  []patchFileSummary
}

type patchFileSummary struct {
	Path      string
	Additions int
	Deletions int
}

func ChangesetRelease(ctx context.Context) error {
	g, err := initAction()
	if err != nil {
		return err
	}

	wf, err := configuration.GetWorkflowAndValidateLanguages(true)
	if err != nil {
		return err
	}

	branchName := cigit.StableBranchName(environment.ActionChangesetRelease)
	var pr *github.PullRequest
	if !environment.IsTestMode() {
		branchName, pr, err = g.FindExistingPR(branchName, environment.ActionChangesetRelease, false)
		if err != nil {
			return err
		}
		branchName, err = g.FindOrCreateBranch(branchName, environment.ActionChangesetRelease)
		if err != nil {
			return err
		}
	}

	if branchName != "" {
		if err := os.Setenv("SPEAKEASY_ACTIVE_BRANCH", branchName); err != nil {
			return fmt.Errorf("failed to set SPEAKEASY_ACTIVE_BRANCH env: %w", err)
		}
	}

	targets, err := collectChangesetReleaseTargets(wf, environment.GetWorkspace())
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		logging.Info("No visible changesets found, skipping changeset release PR")
		return nil
	}

	summaries := make([]changesetReleaseSummary, 0, len(targets))
	for _, target := range targets {
		summary, err := runChangesetReleaseTarget(ctx, g, wf, target)
		if err != nil {
			return err
		}
		summaries = append(summaries, summary)
	}

	if _, err := g.CommitAndPush("", versioninfo.GetSpeakeasyVersion(), "", environment.ActionChangesetRelease, false, nil); err != nil {
		return err
	}

	if environment.IsTestMode() {
		return nil
	}

	title := buildChangesetReleasePRTitle()
	body := buildChangesetReleasePRBody(ctx, g, summaries)

	if err := createOrUpdateChangesetReleasePR(ctx, g, pr, branchName, title, body); err != nil {
		return err
	}

	return nil
}

func collectChangesetReleaseTargets(wf *sdkworkflow.Workflow, workspace string) ([]changesetReleaseTarget, error) {
	targetNames := make([]string, 0, len(wf.Targets))
	for targetName := range wf.Targets {
		targetNames = append(targetNames, targetName)
	}
	sort.Strings(targetNames)

	seenDirs := map[string]string{}
	targets := make([]changesetReleaseTarget, 0, len(targetNames))
	for _, targetName := range targetNames {
		target := wf.Targets[targetName]
		outputDir := "."
		if target.Output != nil && strings.TrimSpace(*target.Output) != "" {
			outputDir = filepath.Clean(*target.Output)
		}

		changesets, err := changeset.LoadAll(filepath.Join(workspace, outputDir))
		if err != nil {
			return nil, err
		}
		if len(changesets) == 0 {
			continue
		}

		if existingTarget, ok := seenDirs[outputDir]; ok {
			return nil, fmt.Errorf("changeset-release currently requires one target per output directory; %s and %s both map to %s", existingTarget, targetName, outputDir)
		}

		seenDirs[outputDir] = targetName
		targets = append(targets, changesetReleaseTarget{
			TargetName: targetName,
			OutputDir:  outputDir,
			Changesets: changesets,
		})
	}

	return targets, nil
}

func runChangesetReleaseTarget(ctx context.Context, g *cigit.Git, wf *sdkworkflow.Workflow, target changesetReleaseTarget) (changesetReleaseSummary, error) {
	summary := changesetReleaseSummary{
		TargetName:  target.TargetName,
		OutputDir:   target.OutputDir,
		BumpType:    highestChangesetReleaseBump(target.Changesets),
		Changesets:  target.Changesets,
		Operations:  collectChangesetOperations(target.Changesets),
		BranchNames: collectChangesetBranches(target.Changesets),
	}

	envUpdates := map[string]string{
		"INPUT_CHANGESET_UPGRADE": "true",
		"INPUT_FORCE":             "true",
		"INPUT_TARGET":            target.TargetName,
	}

	if err := withEnv(envUpdates, func() error {
		_, _, err := run.Run(ctx, g, nil, wf)
		return err
	}); err != nil {
		return summary, err
	}

	cfg, err := config.Load(filepath.Join(environment.GetWorkspace(), target.OutputDir))
	if err != nil {
		return summary, err
	}

	summary.Version = cfg.LockFile.Management.ReleaseVersion
	summary.PatchFiles, err = collectReleasePatchFiles(filepath.Join(environment.GetWorkspace(), target.OutputDir))
	if err != nil {
		return summary, err
	}
	return summary, nil
}

func buildChangesetReleasePRTitle() string {
	title := "chore: version sdk"
	sourceBranch := environment.GetSourceBranch()
	if !environment.IsMainBranch(sourceBranch) {
		title += " [" + environment.SanitizeBranchName(sourceBranch) + "]"
	}
	return title
}

func buildChangesetReleasePRBody(ctx context.Context, g *cigit.Git, summaries []changesetReleaseSummary) string {
	baseBranch := environment.GetTargetBaseBranch()
	baseBranch = strings.TrimPrefix(baseBranch, "refs/heads/")

	var body strings.Builder
	body.WriteString("This PR was opened by `speakeasy ci changeset-release`.\n")
	body.WriteString("When you're ready to release the next SDK version, merge this PR.\n")
	body.WriteString("If more changesets land on `")
	body.WriteString(baseBranch)
	body.WriteString("`, this PR will be updated.\n\n")
	body.WriteString("# Release Summary\n\n")

	for _, summary := range summaries {
		body.WriteString("## ")
		body.WriteString(summary.TargetName)
		body.WriteString("@")
		body.WriteString(summary.Version)
		body.WriteString("\n\n")
		body.WriteString("Version bump: `")
		body.WriteString(summary.BumpType)
		body.WriteString("`\n\n")

		if len(summary.Operations) > 0 {
			body.WriteString("Operations:\n")
			for _, operation := range summary.Operations {
				body.WriteString("- `")
				body.WriteString(operation)
				body.WriteString("`\n")
			}
			body.WriteString("\n")
		}

		if len(summary.PatchFiles) > 0 {
			body.WriteString("Custom code:\n\n")
			body.WriteString("| File | Delta |\n")
			body.WriteString("| --- | --- |\n")
			for _, patchFile := range summary.PatchFiles {
				body.WriteString("| `")
				body.WriteString(patchFile.Path)
				body.WriteString("` | `+")
				body.WriteString(fmt.Sprintf("%d", patchFile.Additions))
				body.WriteString(" -")
				body.WriteString(fmt.Sprintf("%d", patchFile.Deletions))
				body.WriteString("` |\n")
			}
			body.WriteString("\n")
		}
	}

	body.WriteString("# Included Changesets\n\n")
	body.WriteString("| Changeset | Branch | Bump | Summary |\n")
	body.WriteString("| --- | --- | --- | --- |\n")
	for _, summary := range summaries {
		for _, cs := range summary.Changesets {
			body.WriteString("| `")
			body.WriteString(cs.Filename())
			body.WriteString("` | ")
			if cs.Branch != "" {
				body.WriteString("`")
				body.WriteString(cs.Branch)
				body.WriteString("`")
			} else {
				body.WriteString(" ")
			}
			body.WriteString(" | `")
			body.WriteString(string(cs.Version))
			body.WriteString("` | ")

			ops := operationsForChangeset(cs)
			if len(ops) > 0 {
				body.WriteString(strings.Join(ops, ", "))
			} else if title := findPullRequestTitleForBranch(ctx, g.GetClient(), cs.Branch); title != "" {
				body.WriteString(title)
			} else if cs.Message != "" {
				body.WriteString(cs.Message)
			} else {
				body.WriteString(" ")
			}
			body.WriteString(" |\n")
		}
	}

	return body.String()
}

func findPullRequestTitleForBranch(ctx context.Context, client *github.Client, branchName string) string {
	if client == nil || branchName == "" {
		return ""
	}

	prs, _, err := client.PullRequests.List(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), cigit.GetRepo(), &github.PullRequestListOptions{
		State: "all",
		Head:  os.Getenv("GITHUB_REPOSITORY_OWNER") + ":" + branchName,
	})
	if err != nil || len(prs) == 0 {
		return ""
	}

	return prs[0].GetTitle()
}

func createOrUpdateChangesetReleasePR(ctx context.Context, g *cigit.Git, existingPR *github.PullRequest, branchName, title, body string) error {
	prClient := g.GetClient()

	if existingPR != nil {
		existingPR.Title = github.String(title)
		existingPR.Body = github.String(body)
		_, _, err := prClient.PullRequests.Edit(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), cigit.GetRepo(), existingPR.GetNumber(), existingPR)
		if err != nil {
			return fmt.Errorf("failed to update PR: %w", err)
		}
		return nil
	}

	targetBaseBranch := environment.GetTargetBaseBranch()
	if strings.HasPrefix(targetBaseBranch, "refs/") {
		targetBaseBranch = strings.TrimPrefix(targetBaseBranch, "refs/heads/")
	}

	_, _, err := prClient.PullRequests.Create(ctx, os.Getenv("GITHUB_REPOSITORY_OWNER"), cigit.GetRepo(), &github.NewPullRequest{
		Title:               github.String(title),
		Body:                github.String(body),
		Head:                github.String(branchName),
		Base:                github.String(targetBaseBranch),
		MaintainerCanModify: github.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	return nil
}

func highestChangesetReleaseBump(changesets []*changeset.Changeset) string {
	for _, cs := range changesets {
		if cs.Version == changeset.VersionBumpMajor {
			return "major"
		}
	}

	for _, cs := range changesets {
		if cs.Version == changeset.VersionBumpMinor {
			return "minor"
		}
	}

	return "patch"
}

func collectChangesetOperations(changesets []*changeset.Changeset) []string {
	seen := map[string]struct{}{}
	for _, cs := range changesets {
		for _, operation := range operationsForChangeset(cs) {
			seen[operation] = struct{}{}
		}
	}

	operations := make([]string, 0, len(seen))
	for operation := range seen {
		operations = append(operations, operation)
	}
	sort.Strings(operations)

	return operations
}

func collectChangesetBranches(changesets []*changeset.Changeset) []string {
	seen := map[string]struct{}{}
	for _, cs := range changesets {
		if cs.Branch == "" {
			continue
		}
		seen[cs.Branch] = struct{}{}
	}

	branches := make([]string, 0, len(seen))
	for branch := range seen {
		branches = append(branches, branch)
	}
	sort.Strings(branches)

	return branches
}

func collectReleasePatchFiles(outDir string) ([]patchFileSummary, error) {
	patchRoot := filepath.Join(outDir, ".speakeasy", "patches")
	patchFiles := make([]patchFileSummary, 0)
	err := filepath.WalkDir(patchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(patchRoot, path)
		if err != nil {
			return fmt.Errorf("computing relative patch file path: %w", err)
		}
		patchContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading patch file %s: %w", path, err)
		}
		additions, deletions, err := countPatchDelta(relativePath, patchContent)
		if err != nil {
			return fmt.Errorf("parsing patch file %s: %w", relativePath, err)
		}
		patchFiles = append(patchFiles, patchFileSummary{
			Path:      filepath.ToSlash(relativePath),
			Additions: additions,
			Deletions: deletions,
		})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading patch files directory: %w", err)
	}

	sort.Slice(patchFiles, func(i, j int) bool {
		return patchFiles[i].Path < patchFiles[j].Path
	})
	return patchFiles, nil
}

func countPatchDelta(relativePath string, patch []byte) (int, int, error) {
	parsed, errs := diffParser.Parse(ensureDiffGitHeader(relativePath, string(patch)))
	if len(errs) > 0 {
		return 0, 0, errs[0]
	}

	additions := 0
	deletions := 0
	for _, fileDiff := range parsed.FileDiff {
		for _, hunk := range fileDiff.Hunks {
			for _, change := range hunk.ChangeList {
				switch change.Type {
				case diffParser.ContentChangeTypeAdd:
					additions++
				case diffParser.ContentChangeTypeDelete:
					deletions++
				case diffParser.ContentChangeTypeModify:
					additions++
					deletions++
				}
			}
		}
	}

	return additions, deletions, nil
}

func ensureDiffGitHeader(relativePath, patch string) string {
	if strings.HasPrefix(patch, "diff --git ") {
		return patch
	}

	slashPath := filepath.ToSlash(strings.TrimSuffix(relativePath, ".patch"))
	return fmt.Sprintf("diff --git a/%s b/%s\n%s", slashPath, slashPath, patch)
}

func operationsForChangeset(cs *changeset.Changeset) []string {
	if cs == nil || cs.ChangeReport == nil {
		return nil
	}

	operations := make([]string, 0, len(cs.ChangeReport.AddedOperations)+len(cs.ChangeReport.ChangedOperations)+len(cs.ChangeReport.RemovedOperations))
	for _, op := range cs.ChangeReport.AddedOperations {
		operations = append(operations, op.OperationID)
	}
	for _, op := range cs.ChangeReport.ChangedOperations {
		operations = append(operations, op.OperationID)
	}
	for _, op := range cs.ChangeReport.RemovedOperations {
		operations = append(operations, op.OperationID)
	}

	slices.Sort(operations)
	return slices.Compact(operations)
}

func withEnv(values map[string]string, fn func() error) error {
	type previousValue struct {
		value string
		set   bool
	}

	previous := make(map[string]previousValue, len(values))
	for key, value := range values {
		current, ok := os.LookupEnv(key)
		previous[key] = previousValue{value: current, set: ok}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	defer func() {
		for key, value := range previous {
			if value.set {
				_ = os.Setenv(key, value.value)
				continue
			}
			_ = os.Unsetenv(key)
		}
	}()

	return fn()
}
