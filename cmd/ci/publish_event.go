package ci

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type publishEventFlags struct {
	GithubAccessToken string `json:"github-access-token"`
	TargetDirectory   string `json:"target-directory"`
	ActionResult      string `json:"action-result"`
	RegistryName      string `json:"registry-name"`
	Debug             bool   `json:"debug"`
}

var publishEventCmd = &model.ExecutableCommand[publishEventFlags]{
	Usage: "publish-event",
	Short: "Publish SDK event to Speakeasy (used by CI/CD)",
	Long:  "Triggers a publishing telemetry event and marks releases as published.",
	Run:   runPublishEvent,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "target-directory",
			Description:  "Directory of the target SDK",
			DefaultValue: os.Getenv("INPUT_TARGET_DIRECTORY"),
		},
		flag.StringFlag{
			Name:         "action-result",
			Description:  "Result of the publishing action",
			DefaultValue: os.Getenv("GH_ACTION_RESULT"),
		},
		flag.StringFlag{
			Name:         "registry-name",
			Description:  "Name of the package registry",
			DefaultValue: os.Getenv("INPUT_REGISTRY_NAME"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runPublishEvent(ctx context.Context, flags publishEventFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_TARGET_DIRECTORY", flags.TargetDirectory)
	setEnvIfNotEmpty("GH_ACTION_RESULT", flags.ActionResult)
	setEnvIfNotEmpty("INPUT_REGISTRY_NAME", flags.RegistryName)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return fmt.Errorf("ci publish-event: not yet implemented")
}
