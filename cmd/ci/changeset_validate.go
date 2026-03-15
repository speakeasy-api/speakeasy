package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type changesetValidateFlags struct {
	WorkingDirectory string `json:"working-directory"`
	Debug            bool   `json:"debug"`
}

var changesetValidateCmd = &model.ExecutableCommand[changesetValidateFlags]{
	Usage: "changeset-validate",
	Short: "Validate visible changesets for conflicting custom-file lineage",
	Long:  "Validates the visible changesets in the current workspace and fails if multiple unreleased changesets claim the same custom-file lineage head.",
	Run:   runChangesetValidate,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "working-directory",
			Description:  "Working directory for validation",
			DefaultValue: os.Getenv("INPUT_WORKING_DIRECTORY"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug mode",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runChangesetValidate(ctx context.Context, flags changesetValidateFlags) error {
	setEnvIfNotEmpty("INPUT_WORKING_DIRECTORY", flags.WorkingDirectory)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return actions.ChangesetValidate(ctx)
}
