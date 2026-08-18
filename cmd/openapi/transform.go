package openapi

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/transform"
)

var transformCmd = &model.CommandGroup{
	Usage:    "transform",
	Short:    "Transform an OpenAPI spec using a well-defined function",
	Commands: []model.Command{removeUnusedCmd, filterOperationsCmd, cleanupCmd, formatCmd, convertSwaggerCmd, normalizeCmd},
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

type convertSwaggerFlags struct {
	Schema string `json:"schema"`
	Out    string `json:"out"`
}

var convertSwaggerCmd = &model.ExecutableCommand[convertSwaggerFlags]{
	Usage: "convert-swagger",
	Short: "Given a Swagger 2.0 file, convert it to an OpenAPI 3.x file",
	Run:   runConvertSwagger,
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

type formatFlags struct {
	Schema string `json:"schema"`
	Out    string `json:"out"`
	Style  string `json:"style"`
}

var formatCmd = &model.ExecutableCommand[formatFlags]{
	Usage: "format",
	Short: "Format an OpenAPI document using a selected output style",
	Long: "Format an OpenAPI document using either the readable style or the sorted style. " +
		"The sorted style accepts JSON or YAML, emits deterministic JSON, and reorders arrays under required, parameters, oneOf, anyOf, and allOf; " +
		"those array orders can affect generated method signatures, union ordering, or order-sensitive tooling.",
	Run: runFormat,
	Flags: append(basicFlags, flag.EnumFlag{
		Name:          "style",
		Description:   "formatting style to apply (readable or sorted)",
		DefaultValue:  string(workflow.FormatStyleReadable),
		AllowedValues: []string{string(workflow.FormatStyleReadable), string(workflow.FormatStyleSorted)},
	}),
}

var normalizeCmd = &model.ExecutableCommand[normalizeFlags]{
	Usage: "normalize",
	Short: "Normalize an OpenAPI document to be more human-readable",
	Run:   runNormalize,
	Flags: append(basicFlags, []flag.Flag{
		flag.BooleanFlag{
			Name:         "prefixItems",
			Description:  "Normalize prefixItems to be a simple string",
			DefaultValue: false,
		},
	}...),
}

type normalizeFlags struct {
	Schema      string `json:"schema"`
	Out         string `json:"out"`
	PrefixItems bool   `json:"prefixItems"`
}

func runNormalize(ctx context.Context, flags normalizeFlags) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	return transform.NormalizeDocument(ctx, flags.Schema, flags.PrefixItems, yamlOut, out)
}

func runRemoveUnused(ctx context.Context, flags basicFlagsI) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	return transform.RemoveUnused(ctx, flags.Schema, yamlOut, out)
}

func runConvertSwagger(ctx context.Context, flags convertSwaggerFlags) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	return transform.ConvertSwagger(ctx, flags.Schema, yamlOut, out)
}

func runFilterOperations(ctx context.Context, flags filterOperationsFlags) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	return transform.FilterOperations(ctx, flags.Schema, flags.OperationIDs, !flags.Exclude, yamlOut, out)
}

func runCleanup(ctx context.Context, flags basicFlagsI) error {
	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	return transform.CleanupDocument(ctx, flags.Schema, yamlOut, out)
}

func runFormat(ctx context.Context, flags formatFlags) error {
	style := workflow.FormatStyle(flags.Style)
	if style != workflow.FormatStyleReadable && style != workflow.FormatStyleSorted {
		return fmt.Errorf("unsupported format style %q", flags.Style)
	}
	if style == workflow.FormatStyleSorted && utils.HasYAMLExt(strings.ToLower(flags.Out)) {
		return fmt.Errorf("sorted formatting only supports JSON output")
	}

	out, yamlOut, err := setupOutput(ctx, flags.Out)
	if err != nil {
		return err
	}
	defer out.Close()

	if style == workflow.FormatStyleReadable {
		return transform.FormatDocument(ctx, flags.Schema, yamlOut, out)
	}

	return transform.FormatSortedDocument(flags.Schema, yamlOut, out)
}

func setupOutput(_ context.Context, out string) (*os.File, bool, error) {
	yamlOut := utils.HasYAMLExt(strings.ToLower(out))

	if out != "" {
		file, err := os.Create(out)
		if err != nil {
			return nil, yamlOut, err
		}
		return file, yamlOut, nil
	}

	return os.Stdout, yamlOut, nil
}
