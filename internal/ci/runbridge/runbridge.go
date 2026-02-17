// Package runbridge provides a bridge between the CI run package and the speakeasy
// internal run package, replacing the old subprocess-based cli.Run() call with a
// direct Go function call.
package runbridge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/ci/environment"
	"github.com/speakeasy-api/speakeasy/internal/ci/registry"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/versioning-reports/versioning"
)

// RunResults contains the results from a speakeasy run workflow.
type RunResults struct {
	LintingReportURL     string
	ChangesReportURL     string
	OpenAPIChangeSummary string
}

// Run executes the speakeasy run workflow directly via the internal run package,
// replacing the old subprocess-based cli.Run() call.
func Run(ctx context.Context, sourcesOnly bool, installationURLs map[string]string, repoURL string, repoSubdirectories map[string]string, manualVersionBump *versioning.BumpType) (*RunResults, error) {
	// Set environment variables that the old cli.Run() used to set
	if environment.ForceGeneration() {
		fmt.Println("\nforce input enabled - setting SPEAKEASY_FORCE_GENERATION=true")
		os.Setenv("SPEAKEASY_FORCE_GENERATION", "true")
	}

	if manualVersionBump != nil {
		os.Setenv("SPEAKEASY_BUMP_OVERRIDE", string(*manualVersionBump))
	}

	// Create a temp file for the OpenAPI change summary
	changeSummaryFile, err := os.CreateTemp(os.TempDir(), "speakeasy-change-summary")
	if err != nil {
		return nil, fmt.Errorf("error creating change summary file: %w", err)
	}
	os.Setenv("SPEAKEASY_OPENAPI_CHANGE_SUMMARY", changeSummaryFile.Name())
	if err := changeSummaryFile.Close(); err != nil {
		return nil, fmt.Errorf("error closing change summary file: %w", err)
	}

	// Build workflow options matching what the old CLI subprocess would have received
	opts := buildWorkflowOpts(sourcesOnly, installationURLs, repoURL, repoSubdirectories)

	wf, err := run.NewWorkflow(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating workflow: %w", err)
	}

	if err := wf.RunWithVisualization(ctx); err != nil {
		return nil, fmt.Errorf("error running workflow: %w", err)
	}

	// Extract report URLs from workflow source results
	lintingReportURL, changesReportURL := extractReportURLs(wf)

	// Read the change summary file (optional, may not exist on first run)
	changeSummary, _ := os.ReadFile(changeSummaryFile.Name())

	return &RunResults{
		LintingReportURL:     lintingReportURL,
		ChangesReportURL:     changesReportURL,
		OpenAPIChangeSummary: string(changeSummary),
	}, nil
}

func buildWorkflowOpts(sourcesOnly bool, installationURLs map[string]string, repoURL string, repoSubdirectories map[string]string) []run.Opt {
	opts := []run.Opt{}

	if sourcesOnly {
		opts = append(opts, run.WithSource("all"))
	} else {
		specifiedTarget := environment.SpecifiedTarget()
		if specifiedTarget != "" {
			opts = append(opts, run.WithTarget(specifiedTarget))
		} else {
			opts = append(opts, run.WithTarget("all"))
		}
		opts = append(opts, run.WithInstallationURLs(installationURLs))
		opts = append(opts, run.WithRepoSubDirs(repoSubdirectories))
	}

	if repoURL != "" {
		opts = append(opts, run.WithRepo(repoURL))
	}

	tags := registry.ProcessRegistryTags()
	if len(tags) > 0 {
		opts = append(opts, run.WithRegistryTags(tags))
	}

	if environment.SetVersion() != "" {
		opts = append(opts, run.WithSetVersion(environment.SetVersion()))
	}

	// If we are in PR mode we skip testing on generation, this should run as a PR check
	if environment.SkipTesting() || (environment.GetMode() == environment.ModePR && !sourcesOnly) {
		opts = append(opts, run.WithSkipTesting(true))
	}

	if environment.SkipCompile() {
		opts = append(opts, run.WithShouldCompile(false))
	}

	if environment.SkipVersioning() {
		opts = append(opts, run.WithSkipVersioning(true))
	}

	return opts
}

// extractReportURLs iterates over the workflow's source results to find
// linting and changes report URLs that were previously parsed from stdout via regex.
func extractReportURLs(wf *run.Workflow) (lintingReportURL, changesReportURL string) {
	for _, sourceResult := range wf.SourceResults {
		if sourceResult == nil {
			continue
		}
		if lintingReportURL == "" && sourceResult.LintResult != nil && sourceResult.LintResult.Report != nil {
			url := sourceResult.LintResult.Report.URL
			if strings.Contains(url, "app.speakeasy.com") && strings.Contains(url, "linting-report") {
				lintingReportURL = url
			}
		}
		if changesReportURL == "" && sourceResult.ChangeReport != nil {
			url := sourceResult.ChangeReport.URL
			if strings.Contains(url, "app.speakeasy.com") && strings.Contains(url, "changes-report") {
				changesReportURL = url
			}
		}
	}
	return
}
