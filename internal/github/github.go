package github

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/changes"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/markdown"
	"github.com/speakeasy-api/versioning-reports/versioning"
	"go.uber.org/zap"
)

type LintingSummary struct {
	Source    string
	Status    string
	Errors    []error
	ReportURL string
}

type WorkflowSummary interface {
	ToMermaidDiagram() (string, error)
}

const (
	ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

	// 512 KiB. GitHub step summaries have a max size of 1MiB. Both linting and
	// changes reports are written to the same summary. Each report should be
	// truncated below half of the overall limit.
	// Reference: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands#step-isolation-and-limits
	GitHubStepSummaryReportLimit = 512 * 1024

	// Lint report unknown severity
	LintSeverityUnknown = "UNKNOWN"
)

var re = regexp.MustCompile(ansi)

func StripANSICodes(str string) string {
	return re.ReplaceAllString(str, "")
}

func GenerateLintingSummary(ctx context.Context, summary LintingSummary) {
	defer func() {
		if r := recover(); r != nil {
			if env.IsGithubDebugMode() {
				fmt.Printf("::debug::%v\n", r)
			}
		}
	}()

	if !env.IsGithubAction() {
		return
	}

	var summaryMarkdown strings.Builder

	summaryMarkdown.Grow(GitHubStepSummaryReportLimit)
	summaryMarkdown.WriteString("# ")

	if summary.Source != "" {
		summaryMarkdown.WriteString(summary.Source + " ")
	}

	summaryMarkdown.WriteString("Linting Summary\n\n")

	if summary.ReportURL != "" {
		summaryMarkdown.WriteString("Linting report available at <")
		summaryMarkdown.WriteString(summary.ReportURL)
		summaryMarkdown.WriteString(">\n\n")
	}

	summaryMarkdown.WriteString(summary.Status + "\n\n")

	contents := [][]string{}

	contents = append(contents, []string{"Severity", "Type", "Error", "Line"})

	SortErrors(summary.Errors)

	for _, err := range summary.Errors {
		vErr := errors.GetValidationErr(err)
		if vErr != nil {
			contents = append(contents, []string{strings.ToUpper(string(vErr.Severity)), "validation", vErr.Error(), strconv.Itoa(vErr.LineNumber)})
			continue
		}

		uErr := errors.GetUnsupportedErr(err)
		if uErr != nil {
			contents = append(contents, []string{string(errors.SeverityWarn), uErr.Error(), "unsupported", strconv.Itoa(uErr.LineNumber)})
			continue
		}

		contents = append(contents, []string{LintSeverityUnknown, "unknown", err.Error(), ""})
	}

	contentsMarkdown := markdown.CreateMarkdownTable(contents)

	if summaryMarkdown.Len()+len(contentsMarkdown) > GitHubStepSummaryReportLimit {
		summaryMarkdown.WriteString("*The full linting report has been truncated from this summary due to size limits.*\n\n")

		// Truncate hints first to try fitting within size limit.
		contents = slices.DeleteFunc(contents, func(row []string) bool {
			return row[0] == string(errors.SeverityHint)
		})

		contentsMarkdown = markdown.CreateMarkdownTable(contents)
	}

	if summaryMarkdown.Len()+len(contentsMarkdown) > GitHubStepSummaryReportLimit {
		// Truncate warnings next to try fitting within size limit.
		contents = slices.DeleteFunc(contents, func(row []string) bool {
			return row[0] == string(errors.SeverityWarn)
		})

		contentsMarkdown = markdown.CreateMarkdownTable(contents)
	}

	// Only include the errors table if it fits within the size limit.
	if summaryMarkdown.Len()+len(contentsMarkdown) <= GitHubStepSummaryReportLimit {
		summaryMarkdown.WriteString(contentsMarkdown)
	}

	githubactions.AddStepSummary(summaryMarkdown.String())

	// Add linting report to version report for PR description
	var errorCount, warnCount, hintCount int
	for _, err := range summary.Errors {
		vErr := errors.GetValidationErr(err)
		if vErr != nil {
			switch vErr.Severity {
			case errors.SeverityError:
				errorCount++
			case errors.SeverityWarn:
				warnCount++
			case errors.SeverityHint:
				hintCount++
			}
			continue
		}
		if errors.GetUnsupportedErr(err) != nil {
			warnCount++
		}
	}

	var prMD string
	reportLink := ""
	if summary.ReportURL != "" {
		reportLink = "\n\n[View full report](" + summary.ReportURL + ")"
	}
	lintingSummary := fmt.Sprintf("%d errors, %d warnings, %d hints", errorCount, warnCount, hintCount)
	prMD = "<details>\n<summary>Linting Report</summary>\n" + lintingSummary + reportLink + "\n" + "</details>\n"

	_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
		Key:      "linting_report",
		PRReport: prMD,
		Priority: 4, // Slightly lower priority than OpenAPI changes
	})
}

func GenerateChangesSummary(ctx context.Context, url string, summary changes.Summary) {
	defer func() {
		if r := recover(); r != nil {
			if env.IsGithubDebugMode() {
				fmt.Printf("::debug::%v\n", r)
			}
		}
	}()

	if !env.IsGithubAction() {
		return
	}

	var summaryMarkdown strings.Builder

	summaryMarkdown.Grow(GitHubStepSummaryReportLimit)
	summaryMarkdown.WriteString("# API Changes Summary\n\n")

	if url != "" {
		summaryMarkdown.WriteString("Changes report available at <")
		summaryMarkdown.WriteString(url)
		summaryMarkdown.WriteString(">\n\n")
	}

	if summaryMarkdown.Len()+len(summary.Text) > GitHubStepSummaryReportLimit {
		summaryMarkdown.WriteString("*The full changes report has been truncated from this summary due to size limits.*\n\n")
	} else {
		summaryMarkdown.WriteString(summary.Text + "\n\n")
	}

	githubactions.AddStepSummary(StripANSICodes(summaryMarkdown.String()))

	if len(os.Getenv("SPEAKEASY_OPENAPI_CHANGE_SUMMARY")) > 0 {
		filepath := os.Getenv("SPEAKEASY_OPENAPI_CHANGE_SUMMARY")
		f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			log.From(ctx).Warnf("failed to open file %s: %s", filepath, err)
			return
		}

		defer func() {
			if err := f.Close(); err != nil {
				log.From(ctx).Warnf("failed to close file %s: %s", filepath, err)
			}
		}()

		if _, err := f.WriteString(summary.Text); err != nil {
			log.From(ctx).Warnf("failed to write file %s: %s", filepath, err)
			return
		}
		log.From(ctx).Infof("wrote changes summary to \"%s\"", filepath)
	}
	var prMD string
	reportLink := ""
	if url != "" {
		reportLink = "\n\n[View full report](" + url + ")"
	}
	if len(summary.Text) > 0 {
		prMD = "<details>\n<summary>OpenAPI Change Summary</summary>\n" + summary.Text + reportLink + "\n" + "</details>\n"
	} else {
		prMD = "<details>\n<summary>OpenAPI Change Summary</summary>\nNo specification changes" + reportLink + "\n" + "</details>\n"
	}

	// New form -- the above form is deprecated.
	_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
		MustGenerate: summary.Bump != changes.None,
		Key:          "openapi_change_summary",
		PRReport:     prMD,
		Priority:     5, // High priority -- place at top
	})

	// Add execution ID as hidden comment at bottom of PR description
	if cliEvent := events.GetTelemetryEventFromContext(ctx); cliEvent != nil && cliEvent.ExecutionID != "" {
		_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
			Key:      "execution_id",
			PRReport: "<!-- execution_id: " + cliEvent.ExecutionID + " -->",
			Priority: 0, // Lowest priority -- place at bottom
		})
	}
}

func GenerateWorkflowSummary(ctx context.Context, summary WorkflowSummary) {
	defer func() {
		if r := recover(); r != nil {
			if env.IsGithubDebugMode() {
				fmt.Printf("::debug::%v\n", r)
			}
		}
	}()

	if !env.IsGithubAction() {
		return
	}

	logger := log.From(ctx)
	var md string
	chart, err := summary.ToMermaidDiagram()
	if err == nil {
		md = fmt.Sprintf("# Generation Workflow Summary\n\n_This is a breakdown of the 'Generate Target' step above_\n%s", chart)
	} else {
		logger.Error("failed to generate github workflow summary", zap.Error(err))
		md = "# Generation Workflow Summary\n\n:stop_sign: Failed to generate workflow summary. Please try again or [contact support](mailto:support@speakeasy.com)."
	}

	githubactions.AddStepSummary(md)
}

func SortErrors(errs []error) {
	slices.SortStableFunc(errs, func(i, j error) int {
		iVErr := errors.GetValidationErr(i)
		jVErr := errors.GetValidationErr(j)

		switch {
		case iVErr != nil && jVErr != nil:
			switch {
			case iVErr.Severity == errors.SeverityError && jVErr.Severity != errors.SeverityError:
				return -1
			case iVErr.Severity == errors.SeverityWarn && jVErr.Severity == errors.SeverityError:
				return 1
			case iVErr.Severity == errors.SeverityHint && jVErr.Severity != errors.SeverityHint:
				return 1
			}

			switch {
			case iVErr.LineNumber < jVErr.LineNumber:
				return -1
			case iVErr.LineNumber > jVErr.LineNumber:
				return 1
			default:
				return 0
			}
		case iVErr != nil:
			return -1
		case jVErr != nil:
			return 1
		}

		iUErr := errors.GetUnsupportedErr(i)
		jUErr := errors.GetUnsupportedErr(j)

		switch {
		case iUErr != nil && jUErr != nil:
			switch {
			case iUErr.LineNumber < jUErr.LineNumber:
				return -1
			case iUErr.LineNumber > jUErr.LineNumber:
				return 1
			default:
				return 0
			}
		case iUErr != nil:
			return -1
		case jUErr != nil:
			return 1
		}

		return 1
	})
}
