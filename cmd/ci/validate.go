package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type validateFlags struct {
	GithubAccessToken     string   `json:"github-access-token"`
	Specs                 []string `json:"specs"`
	MaxValidationErrors   int      `json:"max-validation-errors"`
	MaxValidationWarnings int      `json:"max-validation-warnings"`
	Ruleset               string   `json:"ruleset"`
	FailOnSkipped         bool     `json:"fail-on-skipped"`
}

var validateCmd = &model.ExecutableCommand[validateFlags]{
	Usage: "validate",
	Short: "Validate OpenAPI specs and post PR comment with results",
	Long:  "Validates OpenAPI specs matching the given glob patterns. Posts a consolidated PR comment with results and writes a GitHub Actions step summary.",
	Run:   runValidate,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for posting PR comments",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringSliceFlag{
			Name:        "specs",
			Shorthand:   "s",
			Description: "Glob patterns for OpenAPI spec files to validate",
			Required:    true,
		},
		flag.IntFlag{
			Name:         "max-validation-errors",
			Description:  "Maximum number of validation errors to display per spec",
			DefaultValue: 1000,
		},
		flag.IntFlag{
			Name:         "max-validation-warnings",
			Description:  "Maximum number of validation warnings to display per spec",
			DefaultValue: 1000,
		},
		flag.StringFlag{
			Name:         "ruleset",
			Shorthand:    "r",
			Description:  "Validation ruleset to use",
			DefaultValue: os.Getenv("INPUT_RULESET"),
		},
		flag.BooleanFlag{
			Name:         "fail-on-skipped",
			Description:  "Fail if any operations would be skipped during SDK generation",
			DefaultValue: os.Getenv("INPUT_FAIL_ON_SKIPPED") == "true",
		},
	},
}

func runValidate(ctx context.Context, flags validateFlags) error {
	ruleset := flags.Ruleset
	if ruleset == "" {
		ruleset = "speakeasy-recommended"
	}

	return actions.ValidateSpecs(ctx, actions.ValidateSpecsInputs{
		GithubAccessToken:     flags.GithubAccessToken,
		Specs:                 flags.Specs,
		MaxValidationErrors:   flags.MaxValidationErrors,
		MaxValidationWarnings: flags.MaxValidationWarnings,
		Ruleset:               ruleset,
		FailOnSkipped:         flags.FailOnSkipped,
	})
}
