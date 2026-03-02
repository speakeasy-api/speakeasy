package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/ci/git"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

// ValidateSpecs discovers specs via glob patterns, validates each, posts a PR comment, and writes a step summary.
func ValidateSpecs(ctx context.Context, specPatterns []string, limits *validation.OutputLimits, ruleset string) error {
	logger := log.From(ctx)

	// Expand globs
	var specPaths []string
	for _, pattern := range specPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		specPaths = append(specPaths, matches...)
	}

	if len(specPaths) == 0 {
		return fmt.Errorf("no spec files found matching patterns: %s", strings.Join(specPatterns, ", "))
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
	hasErrors := false

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
			hasErrors = true
			continue
		}

		res, err := validation.Validate(ctx, logger, schema, specPath, limits, isRemote, ruleset, ".", false, true, "")
		if err != nil {
			results = append(results, github.SpecValidationResult{
				SpecPath: specPath,
				Errors:   []error{err},
			})
			hasErrors = true
			continue
		}

		result := github.SpecValidationResult{
			SpecPath: specPath,
			Errors:   res.Errors,
			Warnings: res.Warnings,
			Hints:    res.Infos,
		}
		results = append(results, result)

		if len(res.Errors) > 0 {
			hasErrors = true
		}
	}

	// Build markdown
	commentBody := github.BuildValidationComment(results)

	// Write step summary
	if env.IsGithubAction() {
		githubactions.AddStepSummary(commentBody)
	}

	// Post/update PR comment
	if env.IsGithubAction() {
		if err := postOrUpdateValidationComment(commentBody); err != nil {
			logger.Warnf("Failed to post PR comment: %s\n", err.Error())
		}
	}

	// Print to stdout for local usage
	fmt.Println(commentBody)

	if hasErrors {
		return fmt.Errorf("one or more OpenAPI specs failed validation")
	}

	return nil
}

func postOrUpdateValidationComment(body string) error {
	accessToken := os.Getenv("INPUT_GITHUB_ACCESS_TOKEN")
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
