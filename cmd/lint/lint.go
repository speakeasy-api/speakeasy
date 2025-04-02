package lint

import (
	"context"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/arazzo"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

const lintLong = "# Lint \n The `lint` command provides a set of commands for linting OpenAPI docs and more."

var LintCmd = &model.CommandGroup{
	Usage:          "lint",
	Aliases:        []string{"validate"},
	Short:          "Lint/Validate OpenAPI documents and Speakeasy configuration files",
	Long:           utils.RenderMarkdown(lintLong),
	InteractiveMsg: "What do you want to lint?",
	Commands:       []model.Command{LintOpenapiCmd, lintConfigCmd, lintArazzoCmd},
}

type LintOpenapiFlags struct {
	SchemaPath            string `json:"schema"`
	Header                string `json:"header"`
	Token                 string `json:"token"`
	MaxValidationErrors   int    `json:"max-validation-errors"`
	MaxValidationWarnings int    `json:"max-validation-warnings"`
	Ruleset               string `json:"ruleset"`
	NonInteractive        bool   `json:"non-interactive"`
}

const lintOpenAPILong = `# Lint 
## OpenAPI

Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.`

var LintOpenapiCmd = &model.ExecutableCommand[LintOpenapiFlags]{
	Usage:          "openapi",
	Short:          "Lint an OpenAPI document",
	Long:           utils.RenderMarkdown(lintOpenAPILong),
	Run:            lintOpenapi,
	RunInteractive: lintOpenapiInteractive,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "local filepath or URL for the OpenAPI schema",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.StringFlag{
			Name:        "header",
			Shorthand:   "H",
			Description: "header key to use if authentication is required for downloading schema from remote URL",
		},
		flag.StringFlag{
			Name:        "token",
			Description: "token value to use if authentication is required for downloading schema from remote URL",
		},
		flag.IntFlag{
			Name:         "max-validation-errors",
			Description:  "limit the number of errors to output (default 1000, 0 = no limit)",
			DefaultValue: 1000,
		},
		flag.IntFlag{
			Name:         "max-validation-warnings",
			Description:  "limit the number of warnings to output (default 1000, 0 = no limit)",
			DefaultValue: 1000,
		},
		flag.StringFlag{
			Name:         "ruleset",
			Shorthand:    "r",
			Description:  "ruleset to use for linting",
			DefaultValue: "speakeasy-recommended",
		},
		flag.BooleanFlag{
			Name:        "non-interactive",
			Description: "force non-interactive mode even when running in a terminal",
		},
	},
}

type lintConfigFlags struct {
	Dir string `json:"dir"`
}

var lintConfigCmd = &model.ExecutableCommand[lintConfigFlags]{
	Usage: "config",
	Short: "Lint a Speakeasy configuration file",
	Long:  `Validates a Speakeasy configuration file for SDK generation.`,
	Run:   lintConfig,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "path to the directory containing the Speakeasy configuration file",
			DefaultValue: ".",
		},
	},
}

type lintArazzoFlags struct {
	File string `json:"file"`
}

var lintArazzoCmd = &model.ExecutableCommand[lintArazzoFlags]{
	Usage: "arazzo",
	Short: "Validate an Arazzo document",
	Long:  `Validates an Arazzo document adheres to the Arazzo specification. Supports either yaml or json based Arazzo documents.`,
	Run:   validateArazzo,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "file",
			Shorthand:    "f",
			Description:  "path to the Arazzo document",
			DefaultValue: "arazzo.yaml",
		},
	},
}

func lintOpenapi(ctx context.Context, flags LintOpenapiFlags) error {
	// no authentication required for validating specs

	limits := validation.OutputLimits{
		MaxWarns:  flags.MaxValidationWarnings,
		MaxErrors: flags.MaxValidationErrors,
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if _, err := validation.ValidateOpenAPI(ctx, "", flags.SchemaPath, flags.Header, flags.Token, &limits, flags.Ruleset, wd, false, false); err != nil {
		return err
	}

	return nil
}

func lintOpenapiInteractive(ctx context.Context, flags LintOpenapiFlags) error {
	// If non-interactive flag is set, use the non-interactive version
	if flags.NonInteractive {
		return lintOpenapi(ctx, flags)
	}

	limits := validation.OutputLimits{
		MaxWarns:  flags.MaxValidationWarnings,
		MaxErrors: flags.MaxValidationErrors,
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if _, err := validation.ValidateWithInteractivity(ctx, flags.SchemaPath, flags.Header, flags.Token, &limits, flags.Ruleset, wd, false); err != nil {
		return err
	}

	return nil
}

func lintConfig(ctx context.Context, flags lintConfigFlags) error {
	// To support the old version of this command, check if there is no workflow.yaml. If there isn't, run the old version
	wf, _, err := utils.GetWorkflowAndDir()
	if wf == nil {
		log.From(ctx).Info("No workflow.yaml found, running legacy version of this command...")
		return sdkgen.ValidateConfig(ctx, flags.Dir)
	}

	// Below is the workflow file based version of this command

	targetToConfig, err := validation.GetAndValidateConfigs(ctx)
	if err != nil {
		return err
	}

	langs := strings.Join(slices.Collect(maps.Keys(targetToConfig)), ", ")

	msg := styles.RenderSuccessMessage(
		"SDK generation configuration is valid ✓",
		"Validated targets: "+langs,
	)

	log.From(ctx).Println(msg)

	return nil
}

func validateArazzo(ctx context.Context, flags lintArazzoFlags) error {
	return arazzo.Validate(ctx, flags.File)
}
