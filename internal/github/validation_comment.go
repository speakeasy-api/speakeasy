package github

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
)

const ValidationCommentMarker = "<!-- speakeasy-validation-comment -->"

// SpecValidationResult holds the validation results for a single OpenAPI spec.
type SpecValidationResult struct {
	SpecPath          string
	Errors            []error
	Warnings          []error
	Hints             []error
	InvalidOperations []string
}

// BuildValidationComment generates a consolidated markdown comment for all spec validation results.
func BuildValidationComment(results []SpecValidationResult) string {
	var md strings.Builder

	md.WriteString(ValidationCommentMarker + "\n")
	md.WriteString("## OpenAPI Spec Validation\n\n")

	if len(results) == 0 {
		md.WriteString("No specs found to validate.\n")
		return md.String()
	}

	// Summary table
	md.WriteString("| Spec | Status | Errors | Warnings | Hints | Skipped Ops |\n")
	md.WriteString("|------|--------|--------|----------|-------|-------------|\n")

	for _, r := range results {
		md.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d |\n",
			r.SpecPath,
			summaryStatus(r),
			len(r.Errors),
			len(r.Warnings),
			len(r.Hints),
			len(r.InvalidOperations),
		))
	}

	md.WriteString("\n")

	// Expandable details for specs with issues
	for _, r := range results {
		if !hasIssues(r) {
			continue
		}

		// Build summary line
		var parts []string
		if len(r.Errors) > 0 {
			parts = append(parts, pluralize(len(r.Errors), "error"))
		}
		if len(r.Warnings) > 0 {
			parts = append(parts, pluralize(len(r.Warnings), "warning"))
		}
		if len(r.Hints) > 0 {
			parts = append(parts, pluralize(len(r.Hints), "hint"))
		}
		if len(r.InvalidOperations) > 0 {
			parts = append(parts, pluralize(len(r.InvalidOperations), "skipped operation"))
		}

		icon := detailIcon(r)
		summary := fmt.Sprintf("%s %s — %s", icon, r.SpecPath, strings.Join(parts, ", "))

		md.WriteString("<details>\n")
		md.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", summary))

		if len(r.Errors) > 0 || len(r.Warnings) > 0 || len(r.Hints) > 0 {
			md.WriteString("| Severity | Rule | Message | Line |\n")
			md.WriteString("|----------|------|---------|------|\n")

			allErrs := append(append(append([]error{}, r.Errors...), r.Warnings...), r.Hints...)
			SortErrors(allErrs)

			for _, err := range allErrs {
				vErr := errors.GetValidationErr(err)
				if vErr != nil {
					md.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
						strings.ToUpper(string(vErr.Severity)),
						vErr.Rule,
						vErr.Message,
						strconv.Itoa(vErr.GetLineNumber())))
					continue
				}

				uErr := errors.GetUnsupportedErr(err)
				if uErr != nil {
					md.WriteString(fmt.Sprintf("| WARN | unsupported | %s | %s |\n",
						uErr.Error(),
						strconv.Itoa(uErr.GetLineNumber())))
					continue
				}

				md.WriteString(fmt.Sprintf("| UNKNOWN | unknown | %s | |\n", err.Error()))
			}
			md.WriteString("\n")
		}

		if len(r.InvalidOperations) > 0 {
			md.WriteString("**Skipped operations** (would be excluded from generated SDK):\n")
			for _, op := range r.InvalidOperations {
				md.WriteString(fmt.Sprintf("- `%s`\n", op))
			}
			md.WriteString("\n")
		}

		md.WriteString("</details>\n\n")
	}

	return md.String()
}

func pluralize(count int, word string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, word)
	}
	return fmt.Sprintf("%d %ss", count, word)
}

func hasIssues(result SpecValidationResult) bool {
	return len(result.Errors) > 0 || len(result.Warnings) > 0 || len(result.Hints) > 0 || len(result.InvalidOperations) > 0
}

func summaryStatus(result SpecValidationResult) string {
	switch {
	case len(result.Errors) > 0:
		return ":x: Invalid"
	case len(result.InvalidOperations) > 0:
		return ":warning: Skipped Ops"
	case len(result.Warnings) > 0:
		return ":warning: Warnings"
	case len(result.Hints) > 0:
		return ":information_source: Hints"
	default:
		return ":white_check_mark: Valid"
	}
}

func detailIcon(result SpecValidationResult) string {
	switch {
	case len(result.Errors) > 0:
		return ":x:"
	case len(result.Warnings) > 0 || len(result.InvalidOperations) > 0:
		return ":warning:"
	case len(result.Hints) > 0:
		return ":information_source:"
	default:
		return ":white_check_mark:"
	}
}
