package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type suggestFlags struct {
	GithubAccessToken    string `json:"github-access-token"`
	MaxSuggestions       string `json:"max-suggestions"`
	SpeakeasyVersion     string `json:"speakeasy-version"`
	OpenAPIDocOutput     string `json:"openapi-doc-output"`
	OpenAPIDocLocation   string `json:"openapi-doc-location"`
	OpenAPIDocAuthHeader string `json:"openapi-doc-auth-header"`
	OpenAPIDocAuthToken  string `json:"openapi-doc-auth-token"`
	Debug                bool   `json:"debug"`
}

var suggestCmd = &model.ExecutableCommand[suggestFlags]{
	Usage: "suggest",
	Short: "Run OpenAPI spec suggestions (used by CI/CD)",
	Long:  "Generates suggestions for improving an OpenAPI spec. Creates a PR branch with suggested changes.",
	Run:   runCISuggest,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "max-suggestions",
			Description:  "Maximum number of suggestions to generate",
			DefaultValue: os.Getenv("INPUT_MAX_SUGGESTIONS"),
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
			Name:         "openapi-doc-location",
			Description:  "Location of the OpenAPI document",
			DefaultValue: os.Getenv("INPUT_OPENAPI_DOC_LOCATION"),
		},
		flag.StringFlag{
			Name:         "openapi-doc-auth-header",
			Description:  "Auth header for remote OpenAPI doc",
			DefaultValue: os.Getenv("INPUT_OPENAPI_DOC_AUTH_HEADER"),
		},
		flag.StringFlag{
			Name:         "openapi-doc-auth-token",
			Description:  "Auth token for remote OpenAPI doc",
			DefaultValue: os.Getenv("INPUT_OPENAPI_DOC_AUTH_TOKEN"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runCISuggest(ctx context.Context, flags suggestFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_MAX_SUGGESTIONS", flags.MaxSuggestions)
	setEnvIfNotEmpty("INPUT_SPEAKEASY_VERSION", flags.SpeakeasyVersion)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_OUTPUT", flags.OpenAPIDocOutput)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_LOCATION", flags.OpenAPIDocLocation)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_AUTH_HEADER", flags.OpenAPIDocAuthHeader)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_AUTH_TOKEN", flags.OpenAPIDocAuthToken)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return actions.Suggest(ctx)
}
