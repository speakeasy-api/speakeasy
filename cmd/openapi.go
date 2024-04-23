package cmd

import (
	"context"
	"fmt"
	"github.com/pb33f/openapi-changes/tui"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/transform"
	"github.com/speakeasy-api/speakeasy/registry"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/changes"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

var openapiCmd = &model.CommandGroup{
	Usage:          "openapi",
	Short:          "Validate and compare OpenAPI documents",
	Long:           `The "openapi" command provides a set of commands for validating and comparing OpenAPI docs.`,
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{openapiValidateCmd, openapiDiffCmd, transformCmd},
}

var transformCmd = &model.CommandGroup{
	Usage:    "transform",
	Short:    "Transform an OpenAPI spec using a well-defined function",
	Commands: []model.Command{removeUnusedCmd},
}

type removeUnusedFlags struct {
	Schema string `json:"schema"`
	Out    string `json:"out"`
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
			Required:                   true,
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
	Format    string `json:"format"`
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
		flag.StringFlag{
			Name:         "output",
			Shorthand:    "o",
			DefaultValue: "-", // stdout
			Description:  "output file",
		},
		flag.EnumFlag{
			Name:          "format",
			Shorthand:     "f",
			Description:   "output format",
			AllowedValues: []string{"summary", "console", "html"},
			DefaultValue:  "summary",
		},
	},
}

func diffOpenapi(ctx context.Context, flags OpenAPIDiffFlags) error {
	if flags.Format == "console" {
		return fmt.Errorf("console not supported outside of interactive terminals")
	}
	return diffOpenapiInteractive(ctx, flags)
}

func runHTML(c changes.Changes, flags OpenAPIDiffFlags, shouldOpen bool) error {
	bytes := c.GetHTMLReport()
	if flags.Output == "-" {
		fmt.Println(string(bytes))
		return nil
	}
	if len(flags.Output) == 0 {
		flags.Output = "report.html"
	}

	err := os.WriteFile(flags.Output, bytes, 0o644)
	if err != nil {
		return err
	}
	fmt.Printf("Report saved to %s\n", flags.Output)
	if shouldOpen {
		return openInBrowser(flags.Output)
	}
	return nil
}

func runSummary(c changes.Changes) error {
	summary, err := c.GetSummary()
	if err != nil {
		return err
	}
	fmt.Println(summary.Text)
	return nil
}

func runConsole(ctx context.Context, c changes.Changes) error {
	version := events.GetSpeakeasyVersionFromContext(ctx)
	app := tui.BuildApplication(c, version)
	if app == nil {
		return errors.New("console is unable to start")
	}
	if err := app.Run(); err != nil {
		return fmt.Errorf("console is unable to start, are you running this inside a container?: %w", err)
	}
	return nil
}

func diffOpenapiInteractive(ctx context.Context, flags OpenAPIDiffFlags) error {
	hasRegistryBundle, oldSchema, newSchema, err := processRegistryBundles(ctx, flags)
	if err != nil {
		return err
	}

	if hasRegistryBundle {
		// Cleanup temp dir if we had used a registry bundle
		defer os.RemoveAll(workflow.GetTempDir())
	}

	changes, err := changes.GetChanges(oldSchema, newSchema)
	if err != nil {
		return err
	}

	switch flags.Format {
	case "summary":
		return runSummary(changes)
	case "html":
		return runHTML(changes, flags, true)
	case "console":
		return runConsole(ctx, changes)
	}
	return fmt.Errorf("invalid output type: %s", flags.Format)
}

func processRegistryBundles(ctx context.Context, flags OpenAPIDiffFlags) (bool, string, string, error) {
	oldSchema := flags.OldSchema
	newSchema := flags.NewSchema
	hasRegistrySchema := false
	if strings.Contains(oldSchema, "registry.speakeasyapi.dev/") {
		document := workflow.Document{
			Location: oldSchema,
		}

		output := document.GetTempRegistryDir(workflow.GetTempDir())
		oldSchemaResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, document, output)
		if err != nil {
			return false, "", "", err
		}
		oldSchema = oldSchemaResult.LocalFilePath

		cliEvent := events.GetTelemetryEventFromContext(ctx)
		if cliEvent != nil {
			cliEvent.OpenapiDiffBaseSourceRevisionDigest = &oldSchemaResult.ManifestDigest
			cliEvent.OpenapiDiffBaseSourceNamespaceName = &oldSchemaResult.NamespaceName
			cliEvent.OpenapiDiffBaseSourceBlobDigest = &oldSchemaResult.BlobDigest
		}

		hasRegistrySchema = true
	}

	if strings.Contains(newSchema, "registry.speakeasyapi.dev/") {
		document := workflow.Document{
			Location: newSchema,
		}

		output := document.GetTempRegistryDir(workflow.GetTempDir())
		newSchemaResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, document, output)
		if err != nil {
			return false, "", "", err
		}
		newSchema = newSchemaResult.LocalFilePath

		cliEvent := events.GetTelemetryEventFromContext(ctx)
		if cliEvent != nil {
			cliEvent.SourceRevisionDigest = &newSchemaResult.ManifestDigest
			cliEvent.SourceNamespaceName = &newSchemaResult.NamespaceName
			cliEvent.SourceBlobDigest = &newSchemaResult.BlobDigest
		}

		hasRegistrySchema = true
	}

	return hasRegistrySchema, oldSchema, newSchema, nil
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
