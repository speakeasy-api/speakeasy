package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type finalizeFlags struct {
	GithubAccessToken string `json:"github-access-token"`
	BranchName        string `json:"branch-name"`
	SpeakeasyVersion  string `json:"speakeasy-version"`
	OpenAPIDocOutput  string `json:"openapi-doc-output"`
	CliOutput         string `json:"cli-output"`
	Debug             bool   `json:"debug"`
}

var finalizeCmd = &model.ExecutableCommand[finalizeFlags]{
	Usage: "finalize",
	Short: "Finalize a suggestion PR (used by CI/CD)",
	Long:  "Finalizes a suggestion by creating a PR from the suggestion branch and posting review comments.",
	Run:   runFinalize,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "branch-name",
			Description:  "Branch name for the suggestion",
			DefaultValue: os.Getenv("INPUT_BRANCH_NAME"),
			Required:     true,
		},
		flag.StringFlag{
			Name:         "speakeasy-version",
			Description:  "Pinned Speakeasy CLI version",
			DefaultValue: os.Getenv("INPUT_SPEAKEASY_VERSION"),
		},
		flag.StringFlag{
			Name:         "openapi-doc-output",
			Description:  "Output path for the modified OpenAPI doc",
			DefaultValue: os.Getenv("INPUT_OPENAPI_DOC_OUTPUT"),
		},
		flag.StringFlag{
			Name:         "cli-output",
			Description:  "CLI output from the suggest step",
			DefaultValue: os.Getenv("INPUT_CLI_OUTPUT"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runFinalize(ctx context.Context, flags finalizeFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_BRANCH_NAME", flags.BranchName)
	setEnvIfNotEmpty("INPUT_SPEAKEASY_VERSION", flags.SpeakeasyVersion)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_OUTPUT", flags.OpenAPIDocOutput)
	setEnvIfNotEmpty("INPUT_CLI_OUTPUT", flags.CliOutput)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return actions.FinalizeSuggestion()
}
