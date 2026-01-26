package sdkchangelog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	changes "github.com/speakeasy-api/openapi-generation/v2/pkg/changes"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
	"github.com/speakeasy-api/versioning-reports/versioning"
)

type Requirements struct {
	OldSpecPath  string
	NewSpecPath  string
	OutDir       string // SDK output directory for generated code
	ProjectDir   string // Directory containing .speakeasy/workflow.yaml (may differ from OutDir for nested SDKs)
	Lang         string
	Verbose      bool
	Target       string
	WorkflowStep *workflowTracking.WorkflowStep
}

// Result contains the output of SDK changelog computation
type Result struct {
	MarkdownContent string
	ReportURL       string
}

func ComputeAndStoreSDKChangelog(ctx context.Context, changelogRequirements Requirements) (result Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			log.From(ctx).Errorf("Panic recovered in ComputeAndStoreSDKChangelog: %v\nStack trace:\n%s", r, stackTrace)
			result = Result{}
			err = fmt.Errorf("panic occurred during SDK changelog generation: %v", r)
		}
	}()

	// Check if we have valid spec paths before proceeding
	if changelogRequirements.OldSpecPath == "" || changelogRequirements.NewSpecPath == "" {
		// If we don't have valid spec paths, skip changelog generation
		log.From(ctx).Infof("Skipping SDK changelog generation - missing spec paths (old: %s, new: %s)",
			changelogRequirements.OldSpecPath, changelogRequirements.NewSpecPath)
		return Result{}, nil
	}

	// Create workflow substep if tracking is available
	var changelogStep *workflowTracking.WorkflowStep
	if changelogRequirements.WorkflowStep != nil {
		changelogStep = changelogRequirements.WorkflowStep.NewSubstep("Computing SDK Changelog")
	}

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: changelogRequirements.OldSpecPath,
		NewSpecPath: changelogRequirements.NewSpecPath,
		OutputDir:   changelogRequirements.OutDir,
		Lang:        changelogRequirements.Lang,
		Verbose:     changelogRequirements.Verbose,
		Logger:      log.From(ctx),
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		log.From(ctx).Errorf("an error occurred while computing changes between prior generation and current generation for %s target (language: %s). error: %s", changelogRequirements.Target, changelogRequirements.Lang, err.Error())
		return Result{}, err
	}
	if len(diff.Changes) == 0 {
		log.From(ctx).Infof("0 changes detected for %s target changelog (language: %s)", changelogRequirements.Target, changelogRequirements.Lang)
		if changelogStep != nil {
			changelogStep.Skip("No changes detected")
		}
		return Result{}, nil
	}

	// Compact markdown for inline PR description
	compactMarkdown := changes.ToMarkdown(diff, changes.DetailLevelCompact)

	// Generate full reports for local debugging
	fullMarkdown := changes.ToMarkdown(diff, changes.DetailLevelFull)
	htmlReport := changes.ToHTML(diff)

	// Write reports and specs to .speakeasy/logs/changes/ for local debugging
	changesDir := filepath.Join(changelogRequirements.ProjectDir, ".speakeasy", "logs", "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		log.From(ctx).Warnf("Failed to create changes directory: %s", err.Error())
	} else {
		// Copy old spec
		if oldSpec, err := os.ReadFile(changelogRequirements.OldSpecPath); err != nil {
			log.From(ctx).Warnf("Failed to read old spec: %s", err.Error())
		} else if err := os.WriteFile(filepath.Join(changesDir, "old.openapi.yaml"), oldSpec, 0o644); err != nil {
			log.From(ctx).Warnf("Failed to write old spec: %s", err.Error())
		}

		// Copy new spec
		if newSpec, err := os.ReadFile(changelogRequirements.NewSpecPath); err != nil {
			log.From(ctx).Warnf("Failed to read new spec: %s", err.Error())
		} else if err := os.WriteFile(filepath.Join(changesDir, "new.openapi.yaml"), newSpec, 0o644); err != nil {
			log.From(ctx).Warnf("Failed to write new spec: %s", err.Error())
		}

		// Write markdown
		if err := os.WriteFile(filepath.Join(changesDir, "changes.md"), []byte(fullMarkdown), 0o644); err != nil {
			log.From(ctx).Warnf("Failed to write SDK changelog markdown: %s", err.Error())
		}

		// Write HTML
		if err := os.WriteFile(filepath.Join(changesDir, "changes.html"), htmlReport, 0o644); err != nil {
			log.From(ctx).Warnf("Failed to write SDK changelog HTML: %s", err.Error())
		}

		log.From(ctx).Infof("SDK changelog written to %s", changesDir)
	}

	// Upload full HTML report to registry
	var reportURL string
	if registry.IsRegistryEnabled(ctx) {
		var uploadStep *workflowTracking.WorkflowStep
		if changelogStep != nil {
			uploadStep = changelogStep.NewSubstep("Uploading SDK Changelog Report")
		}

		report, uploadErr := reports.UploadReport(ctx, htmlReport, shared.TypeChanges)
		if uploadErr == nil {
			reportURL = report.URL
			log.From(ctx).Info(report.Message)
			if uploadStep != nil {
				uploadStep.Succeed()
			}
		} else {
			log.From(ctx).Warnf("Failed to upload SDK changelog report: %s", uploadErr.Error())
			if uploadStep != nil {
				uploadStep.Skip("Upload failed")
			}
		}
	}

	// Store PR description with link to full report
	if changelogStep != nil {
		changelogStep.NewSubstep("Storing PR Description")
	}

	err = storeSDKChangelogForPullRequestDescription(ctx, changelogRequirements.Target, compactMarkdown, reportURL)
	if err != nil {
		// Swallow error so that we dont block generation
		log.From(ctx).Warnf("Error generating new changelog: %s", err.Error())
		return Result{}, err
	}

	if changelogStep != nil {
		changelogStep.Succeed()
	}

	return Result{MarkdownContent: compactMarkdown, ReportURL: reportURL}, nil
}

// target refers to workflow target name
// The version reports written here are read in sdk-generation-action to generate commit message
// and PR description. PR description comes from PRReport and Commit message comes from CommitReport
func storeSDKChangelogForPullRequestDescription(ctx context.Context, target string, markdownContent string, reportURL string) error {
	// Build PR content with optional link to full report
	prContent := markdownContent
	if reportURL != "" {
		prContent += fmt.Sprintf("\n\n[View full SDK changelog](%s)", reportURL)
	}

	// Add Release message
	err := storeKeyValueForPullRequestDescription(ctx, fmt.Sprintf("SDK_CHANGELOG_%s", target), prContent, "pr_report")
	if err != nil {
		log.From(ctx).Warnf("error computing changes: %s", err.Error())
		return err
	}

	// Add Commit message (without the URL link for cleaner commit messages)
	err = storeKeyValueForPullRequestDescription(ctx, fmt.Sprintf("COMMIT_MESSAGE_%s", target), markdownContent, "commit_report")
	if err != nil {
		log.From(ctx).Warnf("error computing changes: %s", err.Error())
		return err
	}
	return nil
}

// A bit of a hack for being able to set and get arbitrary keys in a temp file for populating PR description
// Using version-report machinery
func storeKeyValueForPullRequestDescription(ctx context.Context, key string, report string, reportType string) error {
	if reportType != "pr_report" && reportType != "commit_report" {
		return fmt.Errorf("unknown report type passed -> %s", reportType)
	}
	versionReport := versioning.VersionReport{
		Key: key,
		// Higher number means higher priority. Highest priority comes first in the PR description
		Priority:     6,
		MustGenerate: false,
		BumpType:     versioning.BumpNone,
		NewVersion:   "",
	}
	switch reportType {
	case "pr_report":
		versionReport.PRReport += report
	case "commit_report":
		versionReport.CommitReport += report
	}
	err := versioning.AddVersionReport(ctx, versionReport)
	if err != nil {
		log.From(ctx).Warnf("failed to add version report. key: %s, report: %s, error: %s", key, report, err)
		return err
	}
	return nil
}
