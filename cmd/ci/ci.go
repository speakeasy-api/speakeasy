package ci

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
)

// CICmd is a hidden command group for CI/CD integrations.
// These commands are called by the speakeasy-sdk-generation-action and other CI tools.
var CICmd = &model.CommandGroup{
	Usage:             "ci",
	Short:             "CI/CD integration commands",
	Long:              "Commands used by CI/CD integrations like GitHub Actions. Not intended for direct user invocation.",
	Hidden:            true,
	AllowUnknownFlags: true,
	Commands: []model.Command{
		generateCmd,
		releaseCmd,
		suggestCmd,
		finalizeCmd,
		prDescriptionCmd,
		createOrUpdatePRCmd,
		fanoutFinalizeCmd,
		publishEventCmd,

		tagCmd,
		ciTestCmd,
		logResultCmd,
		resolveBranchCmd,
	},
}
