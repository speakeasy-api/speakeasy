package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"gopkg.in/yaml.v3"
)

type MigrateFlags struct {
	Directory string `json:"directory"`
}

var migrateCmd = &model.ExecutableCommand[MigrateFlags]{
	Usage:  "migrate",
	Short:  "migrate to v15 of the speakeasy workflow + action",
	Long:   "migrate to v15 of the speakeasy workflow + action",
	Hidden: true,
	Run:    migrateFunc,
	Flags: []model.Flag{
		model.StringFlag{
			Name:        "directory",
			Shorthand:   "d",
			Description: "directory to migrate. Expected to contain a `.github/workflows` directory",
			Required:    true,
		},
	},
}

func migrateFunc(ctx context.Context, flags MigrateFlags) error {
	genWorkflowFile := yaml.Unmarshal()
}
