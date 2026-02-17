package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type testFlags struct {
	GithubAccessToken string `json:"github-access-token"`
	Target            string `json:"target"`
	WorkingDirectory  string `json:"working-directory"`
	OutputTests       bool   `json:"output-tests"`
	Debug             bool   `json:"debug"`
}

var ciTestCmd = &model.ExecutableCommand[testFlags]{
	Usage: "test",
	Short: "Run SDK tests (used by CI/CD)",
	Long:  "Runs tests for generated SDKs. Identifies changed targets and runs their test suites.",
	Run:   runCITest,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for PR interactions",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "target",
			Description:  "Specific SDK target to test",
			DefaultValue: os.Getenv("INPUT_TARGET"),
		},
		flag.StringFlag{
			Name:         "working-directory",
			Description:  "Working directory",
			DefaultValue: os.Getenv("INPUT_WORKING_DIRECTORY"),
		},
		flag.BooleanFlag{
			Name:         "output-tests",
			Description:  "Output test results",
			DefaultValue: os.Getenv("INPUT_OUTPUT_TESTS") == "true",
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runCITest(ctx context.Context, flags testFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_TARGET", flags.Target)
	setEnvIfNotEmpty("INPUT_WORKING_DIRECTORY", flags.WorkingDirectory)
	setEnvBool("INPUT_OUTPUT_TESTS", flags.OutputTests)
	setEnvBool("INPUT_DEBUG", flags.Debug)
	return actions.Test(ctx)
}
