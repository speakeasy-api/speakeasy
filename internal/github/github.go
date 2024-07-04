package github

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/changes"
	"github.com/speakeasy-api/versioning-reports/versioning"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/markdown"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
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

func StripANSICodes(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	var re = regexp.MustCompile(ansi)
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
			contents = append(contents, []string{"WARN", uErr.Error(), "unsupported", strconv.Itoa(uErr.LineNumber)})
			continue
		}

		contents = append(contents, []string{"UNKNOWN", "unknown", err.Error(), ""})
	}

	var source string
	if summary.Source != "" {
		source = summary.Source + " "
	}

	md := fmt.Sprintf("# %sLinting Summary", source)
	if summary.ReportURL != "" {
		md += fmt.Sprintf("\n\nLinting report available at <%s>", summary.ReportURL)
	}

	md += fmt.Sprintf("\n\n%s\n\n%s", summary.Status, markdown.CreateMarkdownTable(contents))

	githubactions.AddStepSummary(md)
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

	reportLink := ""
	if url != "" {
		reportLink = fmt.Sprintf("Changes report available at <%s>\n\n", url)
	}

	md := fmt.Sprintf("# API Changes Summary\n%s\n%s", reportLink, summary.Text)

	githubactions.AddStepSummary(StripANSICodes(md))

	if len(os.Getenv("SPEAKEASY_OPENAPI_CHANGE_SUMMARY")) > 0 {
		filepath := os.Getenv("SPEAKEASY_OPENAPI_CHANGE_SUMMARY")
		f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.From(ctx).Warnf("failed to open file %s: %s", filepath, err)
			return
		}

		defer func() {
			if err := f.Close(); err != nil {
				log.From(ctx).Warnf("failed to close file %s: %s", filepath, err)
			}
		}()

		if _, err := f.Write([]byte(summary.Text)); err != nil {
			log.From(ctx).Warnf("failed to write file %s: %s", filepath, err)
			return
		}
		log.From(ctx).Infof("wrote changes summary to \"%s\"", filepath)
	}

	// New form -- the above form is deprecated.
	_ = versioning.AddVersionReport(ctx, versioning.VersionReport{
		MustGenerate: summary.Bump != changes.None,
		Key:          "openapi_change_summary",
		PRReport:     summary.Text,
		Priority:     5, // High priority -- place at top
	})
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
	md := ""
	chart, err := summary.ToMermaidDiagram()
	if err == nil {
		md = fmt.Sprintf("# Generation Workflow Summary\n\n_This is a breakdown of the 'Generate Target' step above_\n%s", chart)
	} else {
		logger.Error("failed to generate github workflow summary", zap.Error(err))
		md = "# Generation Workflow Summary\n\n:stop_sign: Failed to generate workflow summary. Please try again or [contact support](mailto:support@speakeasyapi.dev)."
	}

	githubactions.AddStepSummary(md)
}

func SortErrors(errs []error) {
	slices.SortStableFunc(errs, func(i, j error) int {
		iVErr := errors.GetValidationErr(i)
		jVErr := errors.GetValidationErr(j)

		if iVErr != nil && jVErr != nil {
			if iVErr.Severity == errors.SeverityError && jVErr.Severity != errors.SeverityError {
				return -1
			} else if iVErr.Severity == errors.SeverityWarn && jVErr.Severity == errors.SeverityError {
				return 1
			} else if iVErr.Severity == errors.SeverityHint && jVErr.Severity != errors.SeverityHint {
				return 1
			}

			if iVErr.LineNumber < jVErr.LineNumber {
				return -1
			} else if iVErr.LineNumber > jVErr.LineNumber {
				return 1
			}
			return 0
		} else if iVErr != nil {
			return -1
		} else if jVErr != nil {
			return 1
		}

		iUErr := errors.GetUnsupportedErr(i)
		jUErr := errors.GetUnsupportedErr(j)

		if iUErr != nil && jUErr != nil {
			if iUErr.LineNumber < jUErr.LineNumber {
				return -1
			} else if iUErr.LineNumber > jUErr.LineNumber {
				return 1
			}
			return 0
		} else if iUErr != nil {
			return -1
		} else if jUErr != nil {
			return 1
		}

		return 1
	})
}
