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
	SpecPath string
	Errors   []error
	Warnings []error
	Hints    []error
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
	md.WriteString("| Spec | Status | Errors | Warnings |\n")
	md.WriteString("|------|--------|--------|----------|\n")

	for _, r := range results {
		status := ":white_check_mark: Valid"
		if len(r.Errors) > 0 {
			status = ":x: Invalid"
		}
		md.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
			r.SpecPath, status, len(r.Errors), len(r.Warnings)))
	}

	md.WriteString("\n")

	// Expandable details for specs with issues
	for _, r := range results {
		if len(r.Errors) == 0 && len(r.Warnings) == 0 {
			continue
		}

		summary := fmt.Sprintf(":x: %s — %s, %s",
			r.SpecPath, pluralize(len(r.Errors), "error"), pluralize(len(r.Warnings), "warning"))
		if len(r.Errors) == 0 {
			summary = fmt.Sprintf(":warning: %s — %s",
				r.SpecPath, pluralize(len(r.Warnings), "warning"))
		}

		md.WriteString("<details>\n")
		md.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", summary))
		md.WriteString("| Severity | Rule | Message | Line |\n")
		md.WriteString("|----------|------|---------|------|\n")

		allErrs := append(append([]error{}, r.Errors...), r.Warnings...)
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

		md.WriteString("\n</details>\n\n")
	}

	return md.String()
}

func pluralize(count int, word string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, word)
	}
	return fmt.Sprintf("%d %ss", count, word)
}
