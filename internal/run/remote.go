package run

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

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

func RunGitHub(ctx context.Context, target, version string, force bool) error {
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	genLockID, err := getGenLockID(target)
	if err != nil {
		return fmt.Errorf("failed to get gen lock id: %w", err)
	}

	org, repo, err := getRepo(ctx, genLockID)
	if err != nil {
		return err
	}

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
			return fmt.Errorf("Tried to trigger GitHub action but it never started running")
		}
	}

	return nil
}

func RunGitHubRepos(ctx context.Context, target, version string, force bool, githubRepos string) error {
	if githubRepos == "all" {
		return runGitHubReposAll(ctx, target, version, force)
	}

	// Handle comma-separated repo URLs
	repoURLs := strings.Split(githubRepos, ",")
	logger := log.From(ctx)

	// Track successes and failures
	type repoResult struct {
		repo string
		err  error
	}

	results := make([]repoResult, 0, len(repoURLs))

	for _, repoURL := range repoURLs {
		repoURL = strings.TrimSpace(repoURL)
		if repoURL == "" {
			continue
		}

		// Extract org and repo from GitHub URL
		org, repo, err := parseGitHubRepoURL(repoURL)
		if err != nil {
			logger.Errorf("Invalid repository URL '%s': %v", repoURL, err)
			results = append(results, repoResult{
				repo: repoURL,
				err:  err,
			})
			continue
		}

		repoKey := org + "/" + repo
		logger.Printf("Running SDK generation for GitHub repository: %s\n", repoKey)

		err = runGitHubRepoWithOrgAndRepo(ctx, org, repo, target, version, force)
		results = append(results, repoResult{
			repo: repoKey,
			err:  err,
		})

		if err != nil {
			logger.Errorf("Failed to run SDK generation for %s: %v", repoKey, err)
		}
	}

	// Count successes and failures
	successes := 0
	failures := 0
	for _, result := range results {
		if result.err == nil {
			successes++
		} else {
			failures++
		}
	}

	// Generate summary
	logger.Println("\n---------------------------------")
	logger.Printf("SDK Generation Summary: %d/%d repositories processed successfully\n", successes, len(results))

	if failures > 0 {
		logger.Println("\nFailed repositories:")
		for _, result := range results {
			if result.err != nil {
				logger.Printf("  - %s: %v\n", result.repo, result.err)
			}
		}

		return fmt.Errorf("%d/%d repositories failed", failures, len(results))
	}

	return nil
}

func runGitHubReposAll(ctx context.Context, target, version string, force bool) error {
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	targets, err := sdk.Events.GetTargets(ctx, operations.GetWorkspaceTargetsRequest{})
	if err != nil {
		return fmt.Errorf("failed to query the Speakeasy API for SDKs: %w", err)
	}

	// Map to track unique repositories to avoid duplicates
	uniqueRepos := make(map[string]struct{})

	// Collect all unique GitHub repositories
	for _, target := range targets.TargetSDKList {
		if target.GhActionRepository != nil && target.GhActionOrganization != nil &&
			*target.GhActionRepository != "" && *target.GhActionOrganization != "" {
			repoKey := *target.GhActionOrganization + "/" + *target.GhActionRepository
			uniqueRepos[repoKey] = struct{}{}
		}
	}

	if len(uniqueRepos) == 0 {
		return fmt.Errorf("no GitHub repositories found for this workspace you must install the Speakeasy GitHub app to use this feature")
	}

	logger := log.From(ctx)
	logger.Printf("Found %d GitHub repositories connected to this workspace:\n", len(uniqueRepos))

	// Convert map keys to a slice for sorting
	repos := make([]string, 0, len(uniqueRepos))
	for repoKey := range uniqueRepos {
		repos = append(repos, repoKey)
	}

	// Sort repositories for a consistent display order
	slices.Sort(repos)

	// Print all repositories before starting
	for _, repoKey := range repos {
		logger.Printf("  - %s\n", repoKey)
	}

	// Ask for confirmation if there are many repositories
	if len(repos) > 5 {
		logger.Println("\nYou are about to trigger SDK generation for ALL repositories above.")
		logger.Println("This operation might take a while and consume CI/CD minutes.")

		stopSpinner := interactivity.StartSpinner("Press Ctrl+C to cancel or wait 5 seconds to continue...")

		select {
		case <-time.After(5 * time.Second):
			stopSpinner()
			logger.Println("Continuing with all repositories...")
		case <-ctx.Done():
			stopSpinner()
			return fmt.Errorf("operation cancelled")
		}
	}

	// Track successes and failures
	type repoResult struct {
		repo string
		err  error
	}

	results := make([]repoResult, 0, len(repos))

	// Trigger GitHub actions for each repository
	for _, repoKey := range repos {
		parts := strings.Split(repoKey, "/")
		org := parts[0]
		repo := parts[1]

		logger.Printf("\n\nRunning SDK generation for GitHub repository: %s/%s\n", org, repo)

		err = runGitHubRepoWithOrgAndRepo(ctx, org, repo, target, version, force)
		results = append(results, repoResult{
			repo: repoKey,
			err:  err,
		})

		if err != nil {
			logger.Errorf("Failed to run SDK generation for %s: %v", repoKey, err)
		}
	}

	// Count successes and failures
	successes := 0
	failures := 0
	for _, result := range results {
		if result.err == nil {
			successes++
		} else {
			failures++
		}
	}

	// Generate summary
	logger.Println("\n---------------------------------")
	logger.Printf("SDK Generation Summary: %d/%d repositories processed successfully\n", successes, len(repos))

	if failures > 0 {
		logger.Println("\nFailed repositories:")
		for _, result := range results {
			if result.err != nil {
				logger.Printf("  - %s: %v\n", result.repo, result.err)
			}
		}

		return fmt.Errorf("%d/%d repositories failed", failures, len(repos))
	}

	return nil
}

func runGitHubRepoWithOrgAndRepo(ctx context.Context, org, repo, target, version string, force bool) error {
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	// Check access to the repository
	res, err := sdk.Github.CheckAccess(ctx, operations.CheckGithubAccessRequest{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		return fmt.Errorf("failed to check access to %s/%s: %w", org, repo, err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("GitHub app access check failed for %s/%s. Is the Speakeasy GitHub app installed in the repo? Install at: https://github.com/apps/speakeasy-github", org, repo)
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

	// We don't have a genLockID for this specific repo, so we'll use an empty string
	// and let the server figure it out based on the org and repo
	triggerRequest := shared.GithubTriggerActionRequest{
		GenLockID:  "",
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
		return fmt.Errorf("failed to trigger GitHub action for %s/%s: %w", org, repo, err)
	}
	log.From(ctx).Println("Triggered GitHub action for repo:\n" + "https://github.com/" + org + "/" + repo + "/actions \n")

	var runURL string
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	stopSpinner := interactivity.StartSpinner(fmt.Sprintf("Waiting for GitHub Action to start for %s/%s...", org, repo))
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
				return fmt.Errorf("failed to get GitHub action(s) for %s/%s: %w", org, repo, err)
			}

			hasResponse := actionRes != nil && actionRes.GithubGetActionResponse != nil && actionRes.GithubGetActionResponse.RunURL != nil && *actionRes.GithubGetActionResponse.RunURL != "" && actionRes.GithubGetActionResponse.RunStatus != nil
			if hasResponse && isRunning(*actionRes.GithubGetActionResponse.RunStatus) && *actionRes.GithubGetActionResponse.RunURL != initialActionRunURL {
				runURL = *actionRes.GithubGetActionResponse.RunURL
				stopSpinner()
				log.From(ctx).Println(styles.RenderSuccessMessage(fmt.Sprintf("Successfully Kicked Off Generation Run for %s/%s", org, repo), runURL))
				return nil
			}

		case <-timeoutCh:
			stopSpinner()
			return fmt.Errorf("Tried to trigger GitHub action for %s/%s but it never started running", org, repo)
		}
	}

	return nil
}

func parseGitHubRepoURL(url string) (string, string, error) {
	// Remove any trailing slashes
	url = strings.TrimRight(url, "/")

	// Handle different GitHub URL formats:
	// - https://github.com/organization/repository
	// - git@github.com:organization/repository.git
	// - organization/repository

	var orgRepo string

	if strings.HasPrefix(url, "https://github.com/") {
		orgRepo = strings.TrimPrefix(url, "https://github.com/")
	} else if strings.HasPrefix(url, "git@github.com:") {
		orgRepo = strings.TrimPrefix(url, "git@github.com:")
		if strings.HasSuffix(orgRepo, ".git") {
			orgRepo = strings.TrimSuffix(orgRepo, ".git")
		}
	} else if strings.Count(url, "/") == 1 {
		// Assume it's already in the format "organization/repository"
		orgRepo = url
	} else {
		return "", "", fmt.Errorf("invalid GitHub repository URL format: %s", url)
	}

	parts := strings.Split(orgRepo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GitHub repository format: %s", orgRepo)
	}

	org := parts[0]
	repo := parts[1]

	return org, repo, nil
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
