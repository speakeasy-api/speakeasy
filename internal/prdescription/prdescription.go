// Package prdescription generates PR titles and bodies for SDK update PRs.
// This centralizes PR description formatting so that changes only require CLI updates.
package prdescription

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/speakeasy-api/versioning-reports/versioning"
)

// Input contains all data needed to generate a PR description.
// New fields can be added without breaking compatibility - unknown fields are ignored.
type Input struct {
	// Report URLs
	LintingReportURL string `json:"linting_report_url,omitempty"`
	ChangesReportURL string `json:"changes_report_url,omitempty"`

	// Workflow context
	WorkflowName    string `json:"workflow_name,omitempty"`
	SourceBranch    string `json:"source_branch,omitempty"`
	FeatureBranch   string `json:"feature_branch,omitempty"`
	Target          string `json:"target,omitempty"`           // e.g., "typescript", "python"
	SpecifiedTarget string `json:"specified_target,omitempty"` // Target specified via INPUT_TARGET

	// Generation type flags
	SourceGeneration bool `json:"source_generation,omitempty"`
	DocsGeneration   bool `json:"docs_generation,omitempty"`

	// Version information
	SpeakeasyVersion string `json:"speakeasy_version,omitempty"`
	ManualBump       bool   `json:"manual_bump,omitempty"`

	// Version report data (from SPEAKEASY_VERSION_REPORT_LOCATION)
	VersionReport *versioning.MergedVersionReport `json:"version_report,omitempty"`
}

// Output contains the generated PR title and body.
type Output struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// PR title prefixes
const (
	prTitleSDK   = "chore: 🐝 Update SDK - "
	prTitleSpecs = "chore: 🐝 Update Specs - "
	prTitleDocs  = "chore: 🐝 Update SDK Docs - "
)

// Generate creates a PR title and body from the given input.
func Generate(input Input) (*Output, error) {
	title := buildTitle(input)
	body := buildBody(input)

	return &Output{
		Title: title,
		Body:  body,
	}, nil
}

func buildTitle(input Input) string {
	// Determine title prefix based on generation type
	var title string
	switch {
	case input.DocsGeneration:
		title = prTitleDocs + input.WorkflowName
	case input.SourceGeneration:
		title = prTitleSpecs + input.WorkflowName
	default:
		title = prTitleSDK + input.WorkflowName
		// Add target to title if specified and not already in workflow name
		if input.SpecifiedTarget != "" && !strings.Contains(strings.ToUpper(title), strings.ToUpper(input.SpecifiedTarget)) {
			title += " " + strings.ToUpper(input.SpecifiedTarget)
		}
	}

	// Add branch context for feature branches
	if input.FeatureBranch != "" {
		title += " [" + input.FeatureBranch + "]"
	} else if input.SourceBranch != "" && !isMainBranch(input.SourceBranch) {
		sanitized := sanitizeBranchName(input.SourceBranch)
		title += " [" + sanitized + "]"
	}

	// Add version suffix from version report
	suffix := getVersionSuffix(input.VersionReport)
	title += suffix

	return title
}

func buildBody(input Input) string {
	var body strings.Builder

	// Main heading
	if input.SourceGeneration {
		body.WriteString("Update of compiled sources")
	} else {
		body.WriteString("# SDK update\n")
	}

	// Versioning section
	if input.VersionReport != nil {
		bumpType, _ := getPRBumpType(input.VersionReport)
		if bumpType != "" && bumpType != versioning.BumpNone && bumpType != versioning.BumpCustom {
			body.WriteString("## Versioning\n\n")
			versionMsg := fmt.Sprintf("Version Bump Type: [%s] - ", bumpType)
			if input.ManualBump {
				versionMsg += "\U0001F464 (manual)" // 👤
				versionMsg = "**" + versionMsg + "**"
				versionMsg += fmt.Sprintf("\n\nThis PR will stay on the current version until the %s label is removed and/or modified.", bumpType)
			} else {
				versionMsg += "\U0001F916 (automated)" // 🤖
			}
			body.WriteString(versionMsg + "\n")
		}

		// SDK changelog from version reports
		markdownSection := input.VersionReport.GetMarkdownSection()
		body.WriteString(stripANSICodes(markdownSection))
	}

	// Footer with CLI version
	if !input.SourceGeneration && input.SpeakeasyVersion != "" {
		body.WriteString(fmt.Sprintf("\nBased on [Speakeasy CLI](https://github.com/speakeasy-api/speakeasy) %s\n", input.SpeakeasyVersion))
	}

	return body.String()
}

// getVersionSuffix extracts version number for PR title suffix
func getVersionSuffix(report *versioning.MergedVersionReport) string {
	if report == nil {
		return ""
	}

	var singleVersion string
	multipleVersions := false

	for _, r := range report.Reports {
		if r.NewVersion != "" {
			if singleVersion != "" && singleVersion != r.NewVersion {
				multipleVersions = true
				break
			}
			singleVersion = r.NewVersion
		}
	}

	if multipleVersions || singleVersion == "" {
		return ""
	}

	return " " + singleVersion
}

// getPRBumpType extracts the bump type from version report
func getPRBumpType(report *versioning.MergedVersionReport) (versioning.BumpType, bool) {
	if report == nil {
		return "", false
	}

	var singleBumpType versioning.BumpType
	multipleBumpTypes := false

	for _, r := range report.Reports {
		if r.BumpType != "" && r.BumpType != versioning.BumpNone {
			if singleBumpType != "" && singleBumpType != r.BumpType {
				multipleBumpTypes = true
				break
			}
			singleBumpType = r.BumpType
		}
	}

	if multipleBumpTypes {
		return "", false
	}

	return singleBumpType, singleBumpType != ""
}

// isMainBranch checks if a branch name is a main/master branch
func isMainBranch(branch string) bool {
	branch = strings.ToLower(branch)
	return branch == "main" || branch == "master"
}

// sanitizeBranchName normalizes a branch name for use in PR titles.
// Must match environment.SanitizeBranchName so FindExistingPR can find PRs we create.
func sanitizeBranchName(branch string) string {
	branch = strings.TrimPrefix(branch, "refs/heads/")
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = strings.ReplaceAll(branch, "_", "-")
	branch = strings.ReplaceAll(branch, " ", "-")
	branch = strings.Trim(branch, "-")
	return branch
}

// stripANSICodes removes ANSI escape sequences from text
func stripANSICodes(text string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(text, "")
}
