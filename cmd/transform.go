package cmd

import (
	"context"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"os"
)

var transformCmd = &model.CommandGroup{
	Usage:   "transform",
	Short: "Transform an OpenAPI spec using a well-defined function",
	Commands: []model.Command{removeUnusedCmd},
}

type removeUnusedFlags struct {
	Schema  string `json:"schema"`
	Out     string `json:"out"`
}

var removeUnusedCmd = &model.ExecutableCommand[removeUnusedFlags]{
	Usage: "remove-unused",
	Short: "Given an OpenAPI file, remove all unused options",
	Run:   runRemoveUnused,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "the schema to transform",
			Required: true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "out",
			Shorthand:   "o",
			Description: "write directly to a file instead of stdout",
		},
	},
}


func runRemoveUnused(ctx context.Context, flags removeUnusedFlags) error {
	out := os.Stdout
	if flags.Out != "" {
		file, err := os.Create(flags.Out)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}

	return transform.RemoveUnused(ctx, flags.Schema, out)
}