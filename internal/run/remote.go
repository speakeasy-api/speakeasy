package run

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	speakeasyclientsdkgo "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

var githubActionRunningStatuses = []string{
	"queued",
	"in_progress",
	"requested",
	"waiting",
	"pending",
}

func isRunning(status string) bool {
	return slices.Contains(githubActionRunningStatuses, status)
}

func RunGitHub(ctx context.Context, target, version string, force bool, githubValue string) error {
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	// Handle different GitHub values
	if githubValue == "all" {
		return runGitHubAll(ctx, sdk, version, force)
	} else if githubValue != "" && githubValue != "true" {
		// Handle specific GitHub repos
		return runGitHubSpecificRepos(ctx, sdk, githubValue, version, force)
	}

	genLockID, err := getGenLockID(target)
	if err != nil {
		return fmt.Errorf("failed to get gen lock id: %w", err)
	}

	org, repo, err := getRepo(ctx, genLockID)
	if err != nil {
		return err
	}

	return triggerGitHubAction(ctx, sdk, org, repo, target, genLockID, version, force)
}

func triggerGitHubAction(ctx context.Context, sdk *speakeasyclientsdkgo.Speakeasy, org, repo, target, genLockID, version string, force bool) error {
	res, err := sdk.Github.CheckAccess(ctx, operations.CheckGithubAccessRequest{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		return fmt.Errorf("failed to check access: %w", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("GitHub app access check failed. Is the Speakeasy GitHub app installed in the repo? Install at: https://github.com/apps/speakeasy-github")
	}

	initialAction, _ := sdk.Github.GetAction(ctx, operations.GetGitHubActionRequest{
		Org:        org,
		Repo:       repo,
		TargetName: &target,
	})
	initialActionRunURL := ""
	if initialAction != nil && initialAction.GithubGetActionResponse != nil && initialAction.GithubGetActionResponse.RunURL != nil {
		initialActionRunURL = *initialAction.GithubGetActionResponse.RunURL
	}

	triggerRequest := shared.GithubTriggerActionRequest{
		GenLockID:  genLockID,
		Org:        org,
		RepoName:   repo,
		TargetName: &target,
	}
	if version != "" {
		triggerRequest.SetVersion = &version
	}

	if force {
		triggerRequest.Force = &force
	}

	_, err = sdk.Github.TriggerAction(ctx, triggerRequest)
	if err != nil {
		return fmt.Errorf("failed to trigger GitHub action: %w", err)
	}
	log.From(ctx).Println("Triggered GitHub action for repo:\n" + "https://github.com/" + org + "/" + repo + "/actions \n")

	var runURL string
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	stopSpinner := interactivity.StartSpinner("Waiting for GitHub Action to start...")
	defer stopSpinner()

	timeoutCh := time.After(5 * time.Minute)
	for runURL == "" {
		select {
		case <-ticker.C:
			// Perform the action check
			actionRes, err := sdk.Github.GetAction(ctx, operations.GetGitHubActionRequest{
				Org:        org,
				Repo:       repo,
				TargetName: &target,
			})
			if err != nil {
				stopSpinner()
				return fmt.Errorf("failed to get GitHub action(s): %w", err)
			}

			hasResponse := actionRes != nil && actionRes.GithubGetActionResponse != nil && actionRes.GithubGetActionResponse.RunURL != nil && *actionRes.GithubGetActionResponse.RunURL != "" && actionRes.GithubGetActionResponse.RunStatus != nil
			if hasResponse && isRunning(*actionRes.GithubGetActionResponse.RunStatus) && *actionRes.GithubGetActionResponse.RunURL != initialActionRunURL {
				runURL = *actionRes.GithubGetActionResponse.RunURL
				stopSpinner()
				log.From(ctx).Println(styles.RenderSuccessMessage("Successfully Kicked Off Generation Run", runURL))
				return nil
			}

		case <-timeoutCh:
			stopSpinner()
			return fmt.Errorf("tried to trigger GitHub action but it never started running")
		}
	}

	return nil
}

type RepoInfo struct {
	Org     string
	Repo    string
	Target  string
	LockID  string
	Success bool
}

func runGitHubSpecificRepos(ctx context.Context, sdk *speakeasyclientsdkgo.Speakeasy, repoList string, version string, force bool) error {
	// Parse comma-separated list of repositories
	repos := strings.Split(repoList, ",")

	// Display all specified repositories
	log.From(ctx).Println(styles.MakeBold("You specified the following GitHub repositories:"))
	for i, repo := range repos {
		repo = strings.TrimSpace(repo)
		log.From(ctx).Printf("%d. %s\n", i+1, repo)
	}

	// Ask for confirmation
	confirm := interactivity.SimpleConfirm("Do you want to trigger GitHub actions for these repositories?", true)
	if !confirm {
		log.From(ctx).Println("Operation cancelled")
		return nil
	}

	// For each repository URL, extract the owner and repo name and trigger the action
	for _, repoURL := range repos {
		repoURL = strings.TrimSpace(repoURL)

		// Parse GitHub URL to extract owner and repo
		parts := strings.Split(repoURL, "/")
		if len(parts) < 2 {
			log.From(ctx).Printf("Invalid repository URL format: %s, skipping\n", repoURL)
			continue
		}

		var org, repo string
		if strings.Contains(repoURL, "github.com") {
			// Format: https://github.com/owner/repo
			// Find the index of "github.com" and get the next two parts
			for i, part := range parts {
				if part == "github.com" || part == "github.com:" {
					if i+2 < len(parts) {
						org = parts[i+1]
						repo = parts[i+2]
						// Remove .git suffix if present
						repo = strings.TrimSuffix(repo, ".git")
						break
					}
				}
			}
		} else {
			// Format: owner/repo
			lastTwo := parts[len(parts)-2:]
			if len(lastTwo) == 2 {
				org = lastTwo[0]
				repo = lastTwo[1]
				// Remove .git suffix if present
				repo = strings.TrimSuffix(repo, ".git")
			}
		}

		if org == "" || repo == "" {
			log.From(ctx).Printf("Could not extract owner/repo from URL: %s, skipping\n", repoURL)
			continue
		}

		// Get targets for this repository to find a valid genLockID
		targets, err := sdk.Events.GetTargets(ctx, operations.GetWorkspaceTargetsRequest{})
		if err != nil {
			log.From(ctx).Printf("Failed to query the Speakeasy API for targets: %s\n", err)
			continue
		}

		// Find matching genLockID and target for this repository
		var genLockID, targetName string
		for _, target := range targets.TargetSDKList {
			if target.GhActionRepository != nil && target.GhActionOrganization != nil &&
				*target.GhActionRepository == repo && *target.GhActionOrganization == org &&
				target.GenerateGenLockID != "" && target.GenerateTargetName != nil {
				genLockID = target.GenerateGenLockID
				targetName = *target.GenerateTargetName
				break
			}
		}

		if genLockID == "" {
			log.From(ctx).Printf("No matching target found for repository %s/%s, skipping\n", org, repo)
			continue
		}

		// Trigger the GitHub action for this repository
		log.From(ctx).Printf("Triggering GitHub action for %s/%s...\n", org, repo)
		err = triggerGitHubAction(ctx, sdk, org, repo, targetName, genLockID, version, force)
		if err != nil {
			log.From(ctx).Printf("Error triggering GitHub action for %s/%s: %s\n", org, repo, err)
		}
	}

	return nil
}

func runGitHubAll(ctx context.Context, sdk *speakeasyclientsdkgo.Speakeasy, version string, force bool) error {
	// Get all workspace repositories
	targets, err := sdk.Events.GetTargets(ctx, operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return fmt.Errorf("failed to query the Speakeasy API for targets: %w", err)
	}

	var repos []RepoInfo
	seenRepos := make(map[string]bool)

	// Filter for unique repositories
	for _, target := range targets.TargetSDKList {
		if target.GhActionRepository == nil || target.GhActionOrganization == nil || *target.GhActionRepository == "" || *target.GhActionOrganization == "" {
			continue
		}

		repoKey := *target.GhActionOrganization + "/" + *target.GhActionRepository
		if seenRepos[repoKey] {
			continue
		}

		// Filter out invalid targets
		if target.GenerateGenLockID == "" || target.GenerateTargetName == nil {
			continue
		}

		seenRepos[repoKey] = true
		repos = append(repos, RepoInfo{
			Org:     *target.GhActionOrganization,
			Repo:    *target.GhActionRepository,
			Target:  *target.GenerateTargetName,
			LockID:  target.GenerateGenLockID,
			Success: target.Success != nil && *target.Success,
		})
	}

	if len(repos) == 0 {
		return fmt.Errorf("no GitHub repositories found in this workspace")
	}

	// Display all found repositories
	log.From(ctx).Println(styles.MakeBold("Found the following GitHub repositories in your workspace:"))
	for i, repo := range repos {
		statusSymbol := "✅"
		if !repo.Success {
			statusSymbol = "❌"
		}
		log.From(ctx).Printf("%d. %s %s/%s (Target: %s)\n", i+1, statusSymbol, repo.Org, repo.Repo, repo.Target)
	}

	// Ask for confirmation
	confirm := interactivity.SimpleConfirm("Do you want to trigger GitHub actions for all these repositories?", true)
	if !confirm {
		log.From(ctx).Println("Operation cancelled")
		return nil
	}

	// Trigger actions for all repositories
	for _, repo := range repos {
		log.From(ctx).Printf("Triggering GitHub action for %s/%s...\n", repo.Org, repo.Repo)
		err := triggerGitHubAction(ctx, sdk, repo.Org, repo.Repo, repo.Target, repo.LockID, version, force)
		if err != nil {
			log.From(ctx).Printf("Error triggering GitHub action for %s/%s: %s\n", repo.Org, repo.Repo, err)
		}
	}

	return nil
}

func getGenLockID(target string) (string, error) {
	wf, outDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return "", fmt.Errorf("failed to get workflow file: %w", err)
	}

	wfTarget := wf.Targets[target]
	if wfTarget.Output != nil {
		outDir = *wfTarget.Output
	}

	genLockID := sdkgen.GetGenLockID(outDir)
	if genLockID == nil {
		return "", fmt.Errorf("failed to get genLock ID for target")
	}

	return *genLockID, nil
}

func getRepo(ctx context.Context, genLockID string) (string, string, error) {
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get sdk from context: %w", err)
	}

	targets, err := sdk.Events.GetTargets(ctx, operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return "", "", fmt.Errorf("failed to query the Speakeasy API for SDKs: %w", err)
	}

	var org, repo string
	for _, target := range targets.TargetSDKList {
		if target.GenerateGenLockID == genLockID {
			if target.GhActionRepository == nil || target.GhActionOrganization == nil {
				return "", "", fmt.Errorf("no GitHub repo found for target (has it been run in GitHub yet?)")
			}

			repo = *target.GhActionRepository
			org = *target.GhActionOrganization
			break
		}
	}

	if repo == "" {
		return "", "", fmt.Errorf("no events found for target")
	}

	return org, repo, nil
}
