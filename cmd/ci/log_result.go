package ci

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type logResultFlags struct {
	ActionResult             string `json:"action-result"`
	ActionVersion            string `json:"action-version"`
	ActionStep               string `json:"action-step"`
	Languages                string `json:"languages"`
	TargetType               string `json:"target-type"`
	ResolvedSpeakeasyVersion string `json:"resolved-speakeasy-version"`
	Debug                    bool   `json:"debug"`
}

var logResultCmd = &model.ExecutableCommand[logResultFlags]{
	Usage: "log-result",
	Short: "Log CI action result to Speakeasy (used by CI/CD)",
	Long:  "Sends a log entry to the Speakeasy log proxy API with the result of a CI action step.",
	Run:   runLogResult,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "action-result",
			Description:  "Result of the GitHub Action (success/failure)",
			DefaultValue: os.Getenv("GH_ACTION_RESULT"),
		},
		flag.StringFlag{
			Name:         "action-version",
			Description:  "Version of the GitHub Action",
			DefaultValue: os.Getenv("GH_ACTION_VERSION"),
		},
		flag.StringFlag{
			Name:         "action-step",
			Description:  "Name of the action step",
			DefaultValue: os.Getenv("GH_ACTION_STEP"),
		},
		flag.StringFlag{
			Name:         "languages",
			Description:  "Languages/targets being generated",
			DefaultValue: os.Getenv("INPUT_LANGUAGES"),
		},
		flag.StringFlag{
			Name:         "target-type",
			Description:  "Target type (sdk, docs, etc.)",
			DefaultValue: os.Getenv("TARGET_TYPE"),
		},
		flag.StringFlag{
			Name:         "resolved-speakeasy-version",
			Description:  "Resolved Speakeasy CLI version used",
			DefaultValue: os.Getenv("RESOLVED_SPEAKEASY_VERSION"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runLogResult(ctx context.Context, flags logResultFlags) error {
	setEnvIfNotEmpty("GH_ACTION_RESULT", flags.ActionResult)
	setEnvIfNotEmpty("GH_ACTION_VERSION", flags.ActionVersion)
	setEnvIfNotEmpty("GH_ACTION_STEP", flags.ActionStep)
	setEnvIfNotEmpty("INPUT_LANGUAGES", flags.Languages)
	setEnvIfNotEmpty("TARGET_TYPE", flags.TargetType)
	setEnvIfNotEmpty("RESOLVED_SPEAKEASY_VERSION", flags.ResolvedSpeakeasyVersion)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return fmt.Errorf("ci log-result: not yet implemented")
}
