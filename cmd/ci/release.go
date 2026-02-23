package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type releaseFlags struct {
	GithubAccessToken  string `json:"github-access-token"`
	Target             string `json:"target"`
	WorkingDirectory   string `json:"working-directory"`
	Debug              bool   `json:"debug"`
	RegistryTags       string `json:"registry-tags"`
	EnableSDKChangelog string `json:"enable-sdk-changelog"`
}

var releaseCmd = &model.ExecutableCommand[releaseFlags]{
	Usage: "release",
	Short: "Create GitHub releases for generated SDKs (used by CI/CD)",
	Long:  "Creates GitHub releases based on generation output. Used by CI/CD after SDK generation.",
	Run:   runRelease,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for creating releases",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "target",
			Description:  "Specific SDK target to release",
			DefaultValue: os.Getenv("INPUT_TARGET"),
		},
		flag.StringFlag{
			Name:         "working-directory",
			Description:  "Working directory for the release",
			DefaultValue: os.Getenv("INPUT_WORKING_DIRECTORY"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
		flag.StringFlag{
			Name:         "registry-tags",
			Description:  "Registry tags to apply",
			DefaultValue: os.Getenv("INPUT_REGISTRY_TAGS"),
		},
		flag.StringFlag{
			Name:         "enable-sdk-changelog",
			Description:  "Enable SDK changelog generation",
			DefaultValue: os.Getenv("INPUT_ENABLE_SDK_CHANGELOG"),
		},
	},
}

func runRelease(ctx context.Context, flags releaseFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_TARGET", flags.Target)
	setEnvIfNotEmpty("INPUT_WORKING_DIRECTORY", flags.WorkingDirectory)
	setEnvBool("INPUT_DEBUG", flags.Debug)
	setEnvIfNotEmpty("INPUT_REGISTRY_TAGS", flags.RegistryTags)
	setEnvIfNotEmpty("INPUT_ENABLE_SDK_CHANGELOG", flags.EnableSDKChangelog)

	return actions.Release(ctx)
}
