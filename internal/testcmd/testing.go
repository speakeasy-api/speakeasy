package testcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/openapi/arazzo"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/sdk-gen-config/workspace"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

func ExecuteTargetTesting(ctx context.Context, generator *generate.Generator, workflowTarget workflow.Target, targetName, outDir string) (string, error) {
	testReportURL := ""
	err := events.Telemetry(ctx, shared.InteractionTypeTest, func(ctx context.Context, event *shared.CliEvent) error {
		event.GenerateTargetName = &targetName
		if prReference := os.Getenv("GH_PULL_REQUEST"); prReference != "" {
			formattedPr := reformatPullRequestURL(prReference)
			event.GhPullRequest = &formattedPr
		}

		err := generator.RunTargetTesting(ctx, workflowTarget.Target, outDir)

		foundTestReport := populateRawTestReport(outDir, event)
		genLockID := populateGenLockDetails(outDir, event)
		orgSlug := auth.GetOrgSlugFromContext(ctx)
		workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)
		if foundTestReport && genLockID != "" {
			testReportURL = fmt.Sprintf("https://app.speakeasy.com/org/%s/%s/targets/%s/tests/%s", orgSlug, workspaceSlug, genLockID, event.ExecutionID)
		}
		return err
	})

	return testReportURL, err
}

func CheckTestingEnabled(ctx context.Context) error {
	accountType := auth.GetAccountTypeFromContext(ctx)
	orgSlug := auth.GetOrgSlugFromContext(ctx)
	workspaceSlug := auth.GetWorkspaceSlugFromContext(ctx)
	if accountType == nil {
		return fmt.Errorf("Account type not found. Ensure you are logged in via the `speakeasy auth login` command or SPEAKEASY_API_KEY environment variable.")
	}

	if !slices.Contains([]shared.AccountType{shared.AccountTypeEnterprise, shared.AccountTypeBusiness}, *accountType) {
		return fmt.Errorf("testing is not supported on the %s account tier. Contact %s for more information", *accountType, styles.RenderSalesEmail())
	}

	if ok, _ := auth.HasBillingAddOn(ctx, shared.BillingAddOnSDKTesting); !ok {
		return fmt.Errorf("The SDK testing add-on must be enabled to use testing. Please visit %s", fmt.Sprintf("https://app.speakeasy.com/org/%s/%s/settings/billing", orgSlug, workspaceSlug))
	}

	return nil
}

func CheckTestingAccountType(accountType shared.AccountType) bool {
	return slices.Contains([]shared.AccountType{shared.AccountTypeEnterprise, shared.AccountTypeBusiness}, accountType)
}

// RebuildTests will prepare the arazzo and gen.lock files for a target ready to rebuild tests.
func RebuildTests(ctx context.Context, target, rebuildFlag string, cfg *config.Config) error {
	workspaceResult, _ := workspace.FindWorkspace(filepath.Dir(cfg.ConfigPath), workspace.FindWorkspaceOptions{
		FindFile:  "tests.arazzo.yaml",
		Recursive: true,
	})

	if rebuildFlag == "*" {
		cfg.LockFile.GeneratedTests = nil
		if err := config.SaveLockFile(filepath.Dir(cfg.ConfigPath), cfg.LockFile); err != nil {
			return errors.Wrapf(err, "failed to update lock file for target %s", target)
		}
		if workspaceResult != nil {
			err := os.Remove(workspaceResult.Path)
			if err != nil {
				return errors.Wrapf(err, "failed to remove tests.arazzo.yaml file for target %s", target)
			}
		}

		return nil
	}

	var operations []string
	ops := strings.Split(rebuildFlag, ",")
	for _, op := range ops {
		op = strings.TrimSpace(op)
		if op == "" {
			continue
		}
		operations = append(operations, op)

		cfg.LockFile.GeneratedTests.Delete(op)
	}

	if err := config.SaveLockFile(filepath.Dir(cfg.ConfigPath), cfg.LockFile); err != nil {
		return errors.Wrapf(err, "failed to update lock file for target %s", target)
	}

	f, err := os.OpenFile(workspaceResult.Path, os.O_RDWR, 0o600)
	if err != nil {
		return errors.Wrapf(err, "failed to open tests.arazzo.yaml file for target %s", target)
	}
	defer f.Close()

	a, _, err := arazzo.Unmarshal(ctx, f, arazzo.WithSkipValidation())
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal tests.arazzo.yaml file for target %s", target)
	}

	retainedWorkflows := arazzo.Workflows{}
	for _, workflow := range a.Workflows {
		// If the workflow id isn't present in our list then we retain it
		if !slices.Contains(operations, workflow.WorkflowID) {
			retainedWorkflows = append(retainedWorkflows, workflow)
		}
	}

	a.Workflows = retainedWorkflows

	// Seek to beginning and truncate before writing
	if _, err := f.Seek(0, 0); err != nil {
		return errors.Wrapf(err, "failed to seek to beginning of tests.arazzo.yaml file for target %s", target)
	}
	if err := f.Truncate(0); err != nil {
		return errors.Wrapf(err, "failed to truncate tests.arazzo.yaml file for target %s", target)
	}

	if err := arazzo.Marshal(ctx, a, f); err != nil {
		return errors.Wrapf(err, "failed to marshal tests.arazzo.yaml file for target %s", target)
	}

	return nil
}

func populateRawTestReport(outDir string, event *shared.CliEvent) bool {
	foundTestReport := false
	if res, _ := workspace.FindWorkspace(outDir, workspace.FindWorkspaceOptions{
		FindFile:  "reports/tests.xml",
		Recursive: true,
	}); res != nil && len(res.Data) > 0 {
		testReportContent := string(res.Data)
		event.TestReportRaw = &testReportContent
		foundTestReport = true
	}
	return foundTestReport
}

func populateGenLockDetails(outDir string, event *shared.CliEvent) string {
	if cfg, err := config.Load(outDir); err == nil && cfg.LockFile != nil {
		// The generator marks a testing run's version as internal to avoid a bump
		// So we pull current version of the SDK from the lock file
		currentVersion := cfg.LockFile.Management.ReleaseVersion
		event.GenerateVersion = &currentVersion
		return cfg.LockFile.ID
	}

	return ""
}

func reformatPullRequestURL(url string) string {
	url = strings.Replace(url, "https://api.github.com/repos/", "https://github.com/", 1)
	return strings.Replace(url, "/pulls/", "/pull/", 1)
}
