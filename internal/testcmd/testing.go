package testcmd

import (
	"context"
	"os"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
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
		// TODO: Determine whether we will parse the test report here or in the generator
		return err
	})

	return err
}

func reformatPullRequestURL(url string) string {
	url = strings.Replace(url, "https://api.github.com/repos/", "https://github.com/", 1)
	return strings.Replace(url, "/pulls/", "/pull/", 1)
}
