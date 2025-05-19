package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func RunGitHubRepos(ctx context.Context, target, version string, force bool, githubRepos string) error {
	if githubRepos == "all" {
		return runGitHubForAllRepos(ctx, target, version, force)
	}

	repos := parseGitHubRepos(githubRepos)
	if len(repos) == 0 {
		return fmt.Errorf("no valid GitHub repositories provided")
	}

	for _, repo := range repos {
		if err := runGitHubForRepo(ctx, target, version, force, repo); err != nil {
			return fmt.Errorf("failed to run GitHub action for repo %s: %w", repo, err)
		}
	}

	return nil
}

func parseGitHubRepos(githubRepos string) []string {
	if githubRepos == "" {
		return nil
	}

	parts := strings.Split(githubRepos, ",")
	var repos []string

	for _, part := range parts {
		repo := strings.TrimSpace(part)
		if repo == "" {
			continue
		}

		if strings.HasPrefix(repo, "https://github.com/") {
			repo = strings.TrimPrefix(repo, "https://github.com/")
		}

		if strings.Count(repo, "/") == 1 {
			repos = append(repos, repo)
		}
	}

	return repos
}

func runGitHubForAllRepos(ctx context.Context, target, version string, force bool) error {
	return RunGitHub(ctx, target, version, force)
}

func runGitHubForRepo(ctx context.Context, target, version string, force bool, repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid GitHub repository format: %s, expected org/repo", repo)
	}

	org := parts[0]
	repoName := parts[1]

	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	res, err := sdk.Github.CheckAccess(ctx, operations.CheckGithubAccessRequest{
		Org:  org,
		Repo: repoName,
	})
	if err != nil {
		return fmt.Errorf("failed to check access: %w", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("GitHub app access check failed for %s/%s. Is the Speakeasy GitHub app installed in the repo? Install at: https://github.com/apps/speakeasy-github", org, repoName)
	}

	genLockID, err := getGenLockID(target)
	if err != nil {
		return fmt.Errorf("failed to get gen lock id: %w", err)
	}

	initialAction, _ := sdk.Github.GetAction(ctx, operations.GetGitHubActionRequest{
		Org:        org,
		Repo:       repoName,
		TargetName: &target,
	})
	initialActionRunURL := ""
	if initialAction != nil && initialAction.GithubGetActionResponse != nil && initialAction.GithubGetActionResponse.RunURL != nil {
		initialActionRunURL = *initialAction.GithubGetActionResponse.RunURL
	}

	triggerRequest := shared.GithubTriggerActionRequest{
		GenLockID:  genLockID,
		Org:        org,
		RepoName:   repoName,
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
	log.From(ctx).Println("Triggered GitHub action for repo:\n" + "https://github.com/" + org + "/" + repoName + "/actions \n")

	stopSpinner := interactivity.StartSpinner(fmt.Sprintf("Waiting for GitHub Action to start for %s/%s...", org, repoName))
	defer stopSpinner()

	var runURL string
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(5 * time.Minute)
	for runURL == "" {
		select {
		case <-ticker.C:
			actionRes, err := sdk.Github.GetAction(ctx, operations.GetGitHubActionRequest{
				Org:        org,
				Repo:       repoName,
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
				log.From(ctx).Println(styles.RenderSuccessMessage(fmt.Sprintf("Successfully Kicked Off Generation Run for %s/%s", org, repoName), runURL))
				return nil
			}

		case <-timeoutCh:
			stopSpinner()
			return fmt.Errorf("tried to trigger GitHub action but it never started running")
		}
	}

	return nil
}
