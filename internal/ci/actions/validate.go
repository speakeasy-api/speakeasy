package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

// ValidateSpecsInputs holds the inputs for ValidateSpecs.
type ValidateSpecsInputs struct {
	GithubAccessToken     string
	Specs                 []string
	MaxValidationErrors   int
	MaxValidationWarnings int
	Ruleset               string
	FailOnSkipped         bool
}

// ValidateSpecs discovers specs via glob patterns, validates each, posts a PR comment, and writes a step summary.
func ValidateSpecs(ctx context.Context, inputs ValidateSpecsInputs) error {
	logger := log.From(ctx)

	limits := &validation.OutputLimits{
		MaxErrors: inputs.MaxValidationErrors,
		MaxWarns:  inputs.MaxValidationWarnings,
	}

	specPaths, err := discoverSpecPaths(inputs.Specs)
	if err != nil {
		return err
	}

	// Deduplicate
	seen := make(map[string]bool)
	var uniquePaths []string
	for _, p := range specPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if !seen[abs] {
			seen[abs] = true
			uniquePaths = append(uniquePaths, p)
		}
	}

	// Validate each spec
	var results []github.SpecValidationResult
	totalErrors := 0

	for _, specPath := range uniquePaths {
		logger.Infof("Validating %s...\n", specPath)

		// Use Validate() directly instead of ValidateOpenAPI() to avoid
		// per-spec step summaries (we build our own consolidated summary).
		isRemote, schema, err := openapi.GetSchemaContents(ctx, specPath, "", "")
		if err != nil {
			results = append(results, github.SpecValidationResult{
				SpecPath: specPath,
				Errors:   []error{fmt.Errorf("failed to read spec: %w", err)},
			})
			totalErrors++
			continue
		}

		res, err := validation.Validate(ctx, logger, schema, specPath, limits, isRemote, inputs.Ruleset, ".", true, true, "")
		if err != nil {
			results = append(results, github.SpecValidationResult{
				SpecPath: specPath,
				Errors:   []error{err},
			})
			totalErrors++
			continue
		}

		results = append(results, github.SpecValidationResult{
			SpecPath:          specPath,
			Errors:            res.Errors,
			Warnings:          res.Warnings,
			Hints:             res.Infos,
			InvalidOperations: res.InvalidOperations,
		})

		totalErrors += len(res.Errors)
	}

	// Build markdown
	commentBody := github.BuildValidationComment(results)

	// Write step summary and post/update PR comment
	if env.IsGithubAction() {
		githubactions.AddStepSummary(commentBody)

		if err := postOrUpdateValidationComment(inputs.GithubAccessToken, commentBody); err != nil {
			logger.Warnf("Failed to post PR comment: %s\n", err.Error())
		}
	}

	// Print to stdout for local usage
	fmt.Println(commentBody)

	if totalErrors > 0 {
		return fmt.Errorf("validation failed with %d %s across %d %s",
			totalErrors, pluralWord(totalErrors, "error"),
			len(results), pluralWord(len(results), "spec"))
	}

	if inputs.FailOnSkipped {
		for _, r := range results {
			if len(r.InvalidOperations) > 0 {
				return fmt.Errorf("one or more OpenAPI specs have operations that would be skipped during generation")
			}
		}
	}

	return nil
}

func discoverSpecPaths(patterns []string) ([]string, error) {
	var specPaths []string
	for _, pattern := range patterns {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no spec files found matching pattern %q", pattern)
		}
		specPaths = append(specPaths, matches...)
	}

	return specPaths, nil
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func postOrUpdateValidationComment(accessToken, body string) error {
	if accessToken == "" {
		return fmt.Errorf("no github access token available, skipping PR comment")
	}

	prNumber, err := getPRNumberFromEvent()
	if err != nil {
		return fmt.Errorf("could not determine PR number: %w", err)
	}
	if prNumber == 0 {
		return fmt.Errorf("not a PR event, skipping PR comment")
	}

	g := git.New(accessToken)

	// Find and delete existing validation comment
	comments, _ := g.ListIssueComments(prNumber)
	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), github.ValidationCommentMarker) {
			if err := g.DeleteIssueComment(comment.GetID()); err != nil {
				fmt.Printf("Failed to delete existing validation comment: %s\n", err.Error())
			}
		}
	}

	// Post new comment
	return g.WriteIssueComment(prNumber, body)
}

func getPRNumberFromEvent() (int, error) {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return 0, fmt.Errorf("GITHUB_EVENT_PATH not set")
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read event payload: %w", err)
	}

	var payload struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, fmt.Errorf("failed to parse event payload: %w", err)
	}

	return payload.Number, nil
}
