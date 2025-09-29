package sdkchangelog

import (
	"context"
	"fmt"

	changes "github.com/speakeasy-api/openapi-generation/v2/pkg/changes"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/versioning-reports/versioning"
)

type Requirements struct {
	OldSpecPath string
	NewSpecPath string
	OutDir      string
	Lang        string
	Verbose     bool
	Target      string
}

func ComputeAndStoreSDKChangelog(ctx context.Context, changelogRequirements Requirements) (changelogContent string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.From(ctx).Errorf("Panic recovered in ComputeAndStoreSDKChangelog: %v", r)
			changelogContent = ""
			err = fmt.Errorf("panic occurred during SDK changelog generation: %v", r)
		}
	}()

	// Check if we have valid spec paths before proceeding
	if changelogRequirements.OldSpecPath == "" || changelogRequirements.NewSpecPath == "" {
		// If we don't have valid spec paths, skip changelog generation
		log.From(ctx).Infof("Skipping SDK changelog generation - missing spec paths (old: %s, new: %s)",
			changelogRequirements.OldSpecPath, changelogRequirements.NewSpecPath)
		return "", nil
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
		return "", err
	}
	if len(diff.Changes) == 0 {
		log.From(ctx).Infof("0 changes detected for %s target changelog (language: %s)", changelogRequirements.Target, changelogRequirements.Lang)
		return "", nil
	}
	changelogContent, err = storeSDKChangelogForPullRequestDescription(ctx, changelogRequirements.Target, diff)
	if err != nil {
		// Swallow error so that we dont block generation
		log.From(ctx).Warnf("Error generating new changelog: %s", err.Error())
		return "", err
	}
	return changelogContent, nil
}

// target refers to workflow target name
// The version reports written here are read in sdk-generation-action to generate commit message
// and PR description. PR description comes from PRReport and Commit message comes from CommitReport
func storeSDKChangelogForPullRequestDescription(ctx context.Context, target string, diff changes.SDKDiff) (string, error) {
	// Add Release message
	changelogContent := changes.ToMarkdown(diff)
	err := storeKeyValueForPullRequestDescription(ctx, fmt.Sprintf("SDK_CHANGELOG_%s", target), changelogContent, "pr_report")
	if err != nil {
		log.From(ctx).Warnf("error computing changes: %s", err.Error())
		return "", err
	}

	// Add Commit message
	err = storeKeyValueForPullRequestDescription(ctx, fmt.Sprintf("COMMIT_MESSAGE_%s", target), changelogContent, "commit_report")
	if err != nil {
		log.From(ctx).Warnf("error computing changes: %s", err.Error())
		return "", err
	}
	return changelogContent, nil
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
	if reportType == "pr_report" {
		versionReport.PRReport += report
	} else if reportType == "commit_report" {
		versionReport.CommitReport += report
	}
	err := versioning.AddVersionReport(ctx, versionReport)
	if err != nil {
		log.From(ctx).Warnf("failed to add version report. key: %s, report: %s, error: %s", key, report, err)
		return err
	}
	return nil
}
