package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/ci/configuration"
	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/ci/telemetry"
	"github.com/speakeasy-api/speakeasy/internal/ci/testbridge"
	"golang.org/x/exp/slices"
)

const testReportHeader = "SDK Tests Report"

type TestReport struct {
	Success bool
	URL     string
}

func Test(ctx context.Context) error {
	g, err := initAction()
	if err != nil {
		return err
	}

	if err := SetupEnvironment(); err != nil {
		return fmt.Errorf("failed to setup environment: %w", err)
	}

	wf, err := configuration.GetWorkflowAndValidateLanguages(false)
	if err != nil {
		return err
	}

	// This will only come in via workflow dispatch, we do accept 'all' as a special case
	var testedTargets []string
	if providedTargetName := environment.SpecifiedTarget(); providedTargetName != "" && os.Getenv("GITHUB_EVENT_NAME") == "workflow_dispatch" {
		testedTargets = append(testedTargets, providedTargetName)
	}

	var prNumber *int
	targetLockIDs := make(map[string]string)
	if len(testedTargets) == 0 {
		// We look for all files modified in the PR or Branch to see what SDK targets have been modified
		files, number, err := g.GetChangedFilesForPRorBranch()
		if err != nil {
			fmt.Printf("Failed to get commited files: %s\n", err.Error())
		}

		prNumber = number

		for _, file := range files {
			if strings.Contains(file, "gen.yaml") || strings.Contains(file, "gen.lock") {
				configDir := filepath.Dir(filepath.Dir(file)) // gets out of .speakeasy
				cfg, err := config.Load(filepath.Join(environment.GetWorkspace(), configDir))
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				var genLockID string
				if cfg.LockFile != nil {
					genLockID = cfg.LockFile.ID
				}

				outDir, err := filepath.Abs(configDir)
				if err != nil {
					return err
				}
				for name, target := range wf.Targets {
					targetOutput := ""
					if target.Output != nil {
						targetOutput = *target.Output
					}
					targetOutput, err := filepath.Abs(filepath.Join(environment.GetWorkingDirectory(), targetOutput))
					if err != nil {
						return err
					}
					// If there are multiple SDKs in a workflow we ensure output path is unique
					if targetOutput == outDir && !slices.Contains(testedTargets, name) {
						testedTargets = append(testedTargets, name)
						targetLockIDs[name] = genLockID
					}
				}
			}
		}
	}
	if len(testedTargets) == 0 {
		fmt.Println("No target was provided ... skipping tests")
		return nil
	}

	// we will pretty much never have a test action for multiple targets
	// but if a customer manually setup their triggers in this way, we will run test sequentially for clear output

	testReports := make(map[string]TestReport)
	var errs []error
	for _, target := range testedTargets {
		err := testbridge.RunTest(ctx, target)
		if err != nil {
			errs = append(errs, err)
		}

		testReportURL := ""
		if genLockID, ok := targetLockIDs[target]; ok && genLockID != "" {
			testReportURL = formatTestReportURL(ctx, genLockID)
		} else {
			fmt.Println(fmt.Sprintf("No gen.lock ID found for target %s", target))
		}

		if testReportURL == "" {
			fmt.Println(fmt.Sprintf("No test report URL could be formed for target %s", target))
		} else {
			testReports[target] = TestReport{
				Success: err == nil,
				URL:     testReportURL,
			}
		}
	}

	if len(testReports) > 0 {
		if err := writeTestReportComment(g, prNumber, testReports); err != nil {
			fmt.Println(fmt.Sprintf("Failed to write test report comment: %s\n", err.Error()))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("test failures occured: %w", errors.Join(errs...))
	}

	return nil
}

func formatTestReportURL(ctx context.Context, genLockID string) string {
	executionID := os.Getenv(telemetry.ExecutionKeyEnvironmentVariable)
	if executionID == "" {
		return ""
	}

	if ctx.Value(telemetry.OrgSlugKey) == nil {
		return ""
	}
	orgSlug, ok := ctx.Value(telemetry.OrgSlugKey).(string)
	if !ok {
		return ""
	}

	if ctx.Value(telemetry.WorkspaceSlugKey) == nil {
		return ""
	}
	workspaceSlug, ok := ctx.Value(telemetry.WorkspaceSlugKey).(string)
	if !ok {
		return ""
	}

	return fmt.Sprintf("https://app.speakeasy.com/org/%s/%s/targets/%s/tests/%s", orgSlug, workspaceSlug, genLockID, executionID)
}

func writeTestReportComment(g *git.Git, prNumber *int, testReports map[string]TestReport) error {
	if prNumber == nil {
		return fmt.Errorf("PR number is nil, cannot post comment")
	}

	currentPRComments, _ := g.ListIssueComments(*prNumber)
	for _, comment := range currentPRComments {
		commentBody := comment.GetBody()
		if strings.Contains(commentBody, testReportHeader) {
			if err := g.DeleteIssueComment(comment.GetID()); err != nil {
				fmt.Println(fmt.Sprintf("Failed to delete existing test report comment: %s\n", err.Error()))
			}
		}
	}

	titleComment := fmt.Sprintf("## **%s**\n\n", testReportHeader)

	tableHeader := "| Target | Status | Report |\n|--------|--------|--------|\n"

	var tableRows strings.Builder
	for target, report := range testReports {
		statusEmoji := "✅"
		if !report.Success {
			statusEmoji = "❌"
		}
		tableRows.WriteString(fmt.Sprintf("| %s | <p align='center'>%s</p> | [view report](%s) |\n", target, statusEmoji, report.URL))
	}

	// Combine everything
	body := titleComment + tableHeader + tableRows.String()

	err := g.WriteIssueComment(*prNumber, body)

	return err
}
