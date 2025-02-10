package testcmd

import (
	"context"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/sdk-gen-config/workspace"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
)

func ExecuteTargetTesting(ctx context.Context, generator *generate.Generator, workflowTarget workflow.Target, targetName, outDir string) error {
	err := events.Telemetry(ctx, shared.InteractionTypeTest, func(ctx context.Context, event *shared.CliEvent) error {
		event.GenerateTargetName = &targetName
		if prReference := os.Getenv("GH_PULL_REQUEST"); prReference != "" {
			formattedPr := reformatPullRequestURL(prReference)
			event.GhPullRequest = &formattedPr
		}

		err := generator.RunTargetTesting(ctx, workflowTarget.Target, outDir)

		populateRawTestReport(outDir, event)
		populateGenLockDetails(outDir, event)
		return err
	})

	return err
}

func CheckTestingAccountType(accountType shared.AccountType) bool {
	return slices.Contains([]shared.AccountType{shared.AccountTypeEnterprise, shared.AccountTypeBusiness}, accountType)
}

func populateRawTestReport(outDir string, event *shared.CliEvent) {
	if res, _ := workspace.FindWorkspace(outDir, workspace.FindWorkspaceOptions{
		FindFile:  "reports/tests.xml",
		Recursive: true,
	}); res != nil && len(res.Data) > 0 {
		testReportContent := string(res.Data)
		event.TestReportRaw = &testReportContent
	}
}

func populateGenLockDetails(outDir string, event *shared.CliEvent) {
	if cfg, err := config.Load(outDir); err == nil && cfg.LockFile != nil {
		// The generator marks a testing run's version as internal to avoid a bump
		// So we pull current version of the SDK from the lock file
		currentVersion := cfg.LockFile.Management.ReleaseVersion
		event.GenerateVersion = &currentVersion
	}
}

func reformatPullRequestURL(url string) string {
	url = strings.Replace(url, "https://api.github.com/repos/", "https://github.com/", 1)
	return strings.Replace(url, "/pulls/", "/pull/", 1)
}
