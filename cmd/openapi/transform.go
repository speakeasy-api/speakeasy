package openapi

import (
	"context"
	"os"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

var transformCmd = &model.CommandGroup{
	Usage:    "transform",
	Short:    "Transform an OpenAPI spec using a well-defined function",
	Commands: []model.Command{removeUnusedCmd, filterOperationsCmd, cleanupCmd, formatCmd},
}

type basicFlagsI struct {
	Schema string `json:"schema"`
	Out    string `json:"out"`
}

var basicFlags = []flag.Flag{
	flag.StringFlag{
		Name:                       "schema",
		Shorthand:                  "s",
		Description:                "the schema to transform",
		Required:                   true,
		AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
	},
	flag.StringFlag{
		Name:        "out",
		Shorthand:   "o",
		Description: "write directly to a file instead of stdout",
	},
}

var removeUnusedCmd = &model.ExecutableCommand[basicFlagsI]{
	Usage: "remove-unused",
	Short: "Given an OpenAPI file, remove all unused options",
	Run:   runRemoveUnused,
	Flags: basicFlags,
}

type filterOperationsFlags struct {
	Schema       string   `json:"schema"`
	Out          string   `json:"out"`
	OperationIDs []string `json:"operations"`
	Exclude      bool     `json:"exclude"`
}

var filterOperationsCmd = &model.ExecutableCommand[filterOperationsFlags]{
	Usage: "filter-operations",
	Short: "Given an OpenAPI file, filter down to just the given set of operations",
	Run:   runFilterOperations,
	Flags: append(basicFlags, []flag.Flag{
		flag.StringSliceFlag{
			Name:        "operations",
			Description: "list of operation IDs to include (or exclude)",
			Required:    true,
		},
		flag.BooleanFlag{
			Name:         "exclude",
			Shorthand:    "x",
			Description:  "exclude the given operationIDs, rather than including them",
			DefaultValue: false,
		},
	}...),
}

var cleanupCmd = &model.ExecutableCommand[basicFlagsI]{
	Usage: "cleanup",
	Short: "Cleanup the formatting of a given OpenAPI document",
	Run:   runCleanup,
	Flags: basicFlags,
}

var formatCmd = &model.ExecutableCommand[basicFlagsI]{
	Usage: "format",
	Short: "Format a given OpenAPI document",
	Run:   runFormat,
	Flags: basicFlags,
}

func runRemoveUnused(ctx context.Context, flags basicFlagsI) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	defer out.Close()
	if err != nil {
		return err
	}

	return transform.RemoveUnused(ctx, flags.Schema, yamlOut, out)
}

func runFilterOperations(ctx context.Context, flags filterOperationsFlags) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	defer out.Close()
	if err != nil {
		return err
	}

	return transform.FilterOperations(ctx, flags.Schema, flags.OperationIDs, !flags.Exclude, yamlOut, out)
}

func runCleanup(ctx context.Context, flags basicFlagsI) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	defer out.Close()
	if err != nil {
		return err
	}

	return transform.CleanupDocument(ctx, flags.Schema, yamlOut, out)
}

func runFormat(ctx context.Context, flags basicFlagsI) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	defer out.Close()
	if err != nil {
		return err
	}

	return transform.FormatDocument(ctx, flags.Schema, yamlOut, out)
}

func setupOutput(ctx context.Context, out string) (*os.File, bool, error) {
	yamlOut := utils.HasYAMLExt(out)

	if out != "" {
		file, err := os.Create(out)
		if err != nil {
			return nil, yamlOut, err
		}
		return file, yamlOut, nil
	}

	return os.Stdout, yamlOut, nil
}
