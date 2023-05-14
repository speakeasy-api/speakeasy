package github

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/markdown"
	"golang.org/x/exp/slices"
)

func GenerateSummary(status string, errs []error) {
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

	SortErrors(errs)

	for _, err := range errs {
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

	md := fmt.Sprintf("# Validation Summary\n\n%s\n\n%s", status, markdown.CreateMarkdownTable(contents))

	githubactions.AddStepSummary(md)
}

func SortErrors(errs []error) {
	slices.SortStableFunc(errs, func(i, j error) bool {
		iVErr := errors.GetValidationErr(i)
		jVErr := errors.GetValidationErr(j)

		if iVErr != nil && jVErr != nil {
			if iVErr.Severity == errors.SeverityError && jVErr.Severity == errors.SeverityWarn {
				return true
			} else if iVErr.Severity == errors.SeverityWarn && jVErr.Severity == errors.SeverityError {
				return false
			}

			return iVErr.LineNumber < jVErr.LineNumber
		} else if iVErr != nil {
			return true
		} else if jVErr != nil {
			return false
		}

		iUErr := errors.GetUnsupportedErr(i)
		jUErr := errors.GetUnsupportedErr(j)

		if iUErr != nil && jUErr != nil {
			return iUErr.LineNumber < jUErr.LineNumber
		} else if iUErr != nil {
			return true
		} else if jUErr != nil {
			return false
		}

		return false
	})
}
