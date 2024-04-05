package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"os"
	"os/exec"
	"runtime"

	openapiChanges "github.com/speakeasy-api/openapi-changes/cmd"
	"github.com/speakeasy-api/speakeasy/internal/changes"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/spf13/cobra"
)

var openapiCmd = &model.CommandGroup{
	Usage:          "openapi",
	Short:          "Validate and compare OpenAPI documents",
	Long:           `The "openapi" command provides a set of commands for validating and comparing OpenAPI docs.`,
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{openapiValidateCmd, openapiDiffCmd, transformCmd},
}


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

var openapiValidateCmd = &model.ExecutableCommand[LintOpenapiFlags]{
	Usage:          "validate",
	Short:          lintOpenapiCmd.Short,
	Long:           lintOpenapiCmd.Long,
	Run:            lintOpenapiCmd.Run,
	RunInteractive: lintOpenapiCmd.RunInteractive,
	Flags:          lintOpenapiCmd.Flags,
}

type OpenAPIDiffFlags struct {
	OldSchema string `json:"old"`
	NewSchema string `json:"new"`
	Output    string `json:"output"`
}

var openapiDiffCmd = model.ExecutableCommand[OpenAPIDiffFlags]{
	Usage:          "diff",
	Short:          "Visualize the changes between two OpenAPI documents",
	Long:           `Visualize the changes between two OpenAPI documents`,
	Run:            diffOpenapi,
	RunInteractive: diffOpenapiInteractive,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "old",
			Description:                "local filepath or URL for the base OpenAPI schema to compare against",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:                       "new",
			Description:                "local filepath or URL for the updated OpenAPI schema",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.EnumFlag{
			Name:          "output",
			Shorthand:     "o",
			Description:   "how to visualize the diff",
			AllowedValues: []string{"summary", "console", "html"},
			DefaultValue:  "summary",
		},
	},
}

func diffOpenapi(ctx context.Context, flags OpenAPIDiffFlags) error {
	switch flags.Output {
	case "summary":
		return changes.RunSummary(flags.OldSchema, flags.NewSchema)
	case "html":
		return runHTMLReport(flags, false)
	case "console":
		return fmt.Errorf("console not supported outside of interactive terminals")
	}

	return fmt.Errorf("invalid output type: %s", flags.Output)
}

func diffOpenapiInteractive(ctx context.Context, flags OpenAPIDiffFlags) error {
	switch flags.Output {
	case "summary":
		return changes.RunSummary(flags.OldSchema, flags.NewSchema)
	case "html":
		return runHTMLReport(flags, true)
	case "console":
		return runCommand(openapiChanges.GetConsoleCommand(), flags)
	}

	return fmt.Errorf("invalid output type: %s", flags.Output)
}

func runHTMLReport(flags OpenAPIDiffFlags, shouldOpen bool) error {
	err := runCommand(openapiChanges.GetHTMLReportCommand(), flags)
	if err != nil {
		return err
	}

	if shouldOpen {
		return openInBrowser("report.html")
	}

	return nil
}

func runCommand(cmd *cobra.Command, flags OpenAPIDiffFlags) error {
	return cmd.RunE(cmd, []string{flags.OldSchema, flags.NewSchema})
}

func openInBrowser(path string) error {
	var err error

	url := path

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}
