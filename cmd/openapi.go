package cmd

import (
	"context"
	"fmt"
	openapiChanges "github.com/speakeasy-api/openapi-changes/cmd"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/spf13/cobra"
	"os/exec"
	"runtime"
)

var openapiCmd = &model.CommandGroup{
	Usage:          "openapi",
	Short:          "Validate and compare OpenAPI documents",
	Long:           `The "validate" command provides a set of commands for validating OpenAPI docs and more.`,
	InteractiveMsg: "What do you want to validate?",
	Commands:       []model.Command{openapiValidateCmd, openapiDiffCmd},
}

var openapiValidateCmd = &model.ExecutableCommand[ValidateOpenapiFlags]{
	Usage:          "validate",
	Short:          validateOpenapiCmd.Short,
	Long:           validateOpenapiCmd.Long,
	Run:            validateOpenapiCmd.Run,
	RunInteractive: validateOpenapiCmd.RunInteractive,
	Flags:          validateOpenapiCmd.Flags,
}

type OpenAPIDiffFlags struct {
	LeftSchema  string `json:"left"`
	RightSchema string `json:"right"`
	Output      string `json:"output"`
}

var openapiDiffCmd = model.ExecutableCommand[OpenAPIDiffFlags]{
	Usage:          "diff",
	Short:          "Visualize the openapiChanges between two OpenAPI documents",
	Long:           `Visualize the openapiChanges between two OpenAPI documents`,
	Run:            diffOpenapi,
	RunInteractive: diffOpenapiInteractive,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "left",
			Shorthand:                  "l",
			Description:                "local filepath or URL for the OpenAPI schema",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:                       "right",
			Shorthand:                  "r",
			Description:                "local filepath or URL for the OpenAPI schema",
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
		return runCommand(openapiChanges.GetSummaryCommand(), flags)
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
		return runCommand(openapiChanges.GetSummaryCommand(), flags)
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
	return cmd.RunE(cmd, []string{flags.LeftSchema, flags.RightSchema})
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
