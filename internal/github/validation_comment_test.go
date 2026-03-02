package github_test

import (
	"fmt"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestBuildValidationComment_AllPassing(t *testing.T) {
	t.Parallel()

	results := []github.SpecValidationResult{
		{
			SpecPath: "specs/ai-assistant/openapi.yaml",
			Errors:   nil,
			Warnings: nil,
			Hints:    nil,
		},
		{
			SpecPath: "specs/oauth-helper/openapi.yaml",
			Errors:   nil,
			Warnings: nil,
			Hints:    nil,
		},
	}

	comment := github.BuildValidationComment(results)

	assert.Contains(t, comment, github.ValidationCommentMarker)
	assert.Contains(t, comment, "## OpenAPI Spec Validation")
	assert.Contains(t, comment, "specs/ai-assistant/openapi.yaml")
	assert.Contains(t, comment, "specs/oauth-helper/openapi.yaml")
	assert.Contains(t, comment, ":white_check_mark:")
	assert.NotContains(t, comment, ":x:")
	assert.NotContains(t, comment, "<details>")
}

func TestBuildValidationComment_WithFailures(t *testing.T) {
	t.Parallel()

	results := []github.SpecValidationResult{
		{
			SpecPath: "specs/ai-assistant/openapi.yaml",
			Errors:   nil,
			Warnings: nil,
			Hints:    nil,
		},
		{
			SpecPath: "specs/oauth-helper/openapi.yaml",
			Errors: []error{
				errors.NewValidationError("missing-description", &yaml.Node{Line: 42, Column: 5}, fmt.Errorf("Operation must have a description")),
				errors.NewValidationError("invalid-schema", &yaml.Node{Line: 87, Column: 3}, fmt.Errorf("Schema is invalid")),
			},
			Warnings: []error{
				errors.NewValidationWarning("unused-component", &yaml.Node{Line: 15, Column: 1}, fmt.Errorf("Component is unused")),
			},
			Hints: nil,
		},
	}

	comment := github.BuildValidationComment(results)

	assert.Contains(t, comment, github.ValidationCommentMarker)
	assert.Contains(t, comment, ":white_check_mark:")
	assert.Contains(t, comment, ":x:")
	assert.Contains(t, comment, "<details>")
	assert.Contains(t, comment, "specs/oauth-helper/openapi.yaml")
	assert.Contains(t, comment, "2 errors")
	assert.Contains(t, comment, "1 warning")
	assert.Contains(t, comment, "missing-description")
	assert.Contains(t, comment, "42")
}

func TestBuildValidationComment_Empty(t *testing.T) {
	t.Parallel()

	results := []github.SpecValidationResult{}
	comment := github.BuildValidationComment(results)

	assert.Contains(t, comment, github.ValidationCommentMarker)
	assert.Contains(t, comment, "No specs found")
}
