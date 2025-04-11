package cmd

import (
	"context"

	"github.com/speakeasy-api/speakeasy/internal/migrate"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
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
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "directory",
			Shorthand:    "d",
			Description:  "directory to migrate. Expected to contain a `.github/workflows` directory. Defaults to `.`",
			DefaultValue: ".",
		},
	},
}

func migrateFunc(ctx context.Context, flags MigrateFlags) error {
	return migrate.Migrate(ctx, flags.Directory)
}
