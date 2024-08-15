package run

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/config"
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

	res, err := sdk.Github.CheckAccess(ctx, operations.CheckAccessRequest{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		return fmt.Errorf("failed to check access: %w", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("GitHub app access checked failed. Is the Speakeasy GitHub app installed in the repo?")
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

	var runURL string
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(5 * time.Minute)

	for runURL == "" {
		select {
		case <-ticker.C:
			// Perform the action check
			actionRes, err := sdk.Github.GetAction(ctx, operations.GetActionRequest{
				Org:        org,
				Repo:       repo,
				TargetName: &target,
			})
			if err != nil {
				return fmt.Errorf("failed to get GitHub action(s): %w", err)
			}

			if actionRes != nil && actionRes.GithubGetActionResponse != nil && actionRes.GithubGetActionResponse.RunURL != nil && slices.Contains(githubActionRunningStatuses, *actionRes.GithubGetActionResponse.RunStatus) {
				runURL = *actionRes.GithubGetActionResponse.RunURL
				break
			}

		case <-timeoutCh:
			return nil
		}
	}

	log.From(ctx).Println(styles.RenderSuccessMessage("Successfully Kicked Off Generation Run", runURL))

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

	targets, err := sdk.Events.GetWorkspaceTargets(ctx, operations.GetWorkspaceTargetsRequest{
		WorkspaceID: pointer.ToString(config.GetWorkspaceID()),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get workspace targets: %w", err)
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
