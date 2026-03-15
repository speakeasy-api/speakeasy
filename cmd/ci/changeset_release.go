package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type changesetReleaseFlags struct {
	GithubAccessToken  string `json:"github-access-token"`
	WorkingDirectory   string `json:"working-directory"`
	Debug              bool   `json:"debug"`
	SkipCompile        bool   `json:"skip-compile"`
	SkipTesting        bool   `json:"skip-testing"`
	EnableSDKChangelog string `json:"enable-sdk-changelog"`
}

var changesetReleaseCmd = &model.ExecutableCommand[changesetReleaseFlags]{
	Usage: "changeset-release",
	Short: "Create or update the aggregated changeset release PR (used by CI/CD)",
	Long:  "Collapses visible changesets into the next authoritative release snapshot and creates or updates the release PR.",
	Run:   runChangesetRelease,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for repository operations",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "working-directory",
			Description:  "Working directory for the release preparation",
			DefaultValue: os.Getenv("INPUT_WORKING_DIRECTORY"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-compile",
			Description:  "Skip compilation step after generation",
			DefaultValue: os.Getenv("INPUT_SKIP_COMPILE") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-testing",
			Description:  "Skip testing step after generation",
			DefaultValue: os.Getenv("INPUT_SKIP_TESTING") == "true",
		},
		flag.StringFlag{
			Name:         "enable-sdk-changelog",
			Description:  "Enable SDK changelog generation",
			DefaultValue: os.Getenv("INPUT_ENABLE_SDK_CHANGELOG"),
		},
	},
}

func runChangesetRelease(ctx context.Context, flags changesetReleaseFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_WORKING_DIRECTORY", flags.WorkingDirectory)
	setEnvBool("INPUT_DEBUG", flags.Debug)
	setEnvBool("INPUT_SKIP_COMPILE", flags.SkipCompile)
	setEnvBool("INPUT_SKIP_TESTING", flags.SkipTesting)
	setEnvIfNotEmpty("INPUT_ENABLE_SDK_CHANGELOG", flags.EnableSDKChangelog)
	setEnvBool("INPUT_CHANGESET_UPGRADE", true)

	return actions.ChangesetRelease(ctx)
}
