package cmd

import (
	"context"
	"fmt"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

var lintCmd = &model.CommandGroup{
	Usage:          "lint",
	Aliases:        []string{"validate"},
	Short:          "Lint OpenAPI documents + more",
	Long:           `The "lint" command provides a set of commands for linting OpenAPI docs and more.`,
	InteractiveMsg: "What do you want to lint?",
	Commands:       []model.Command{lintOpenapiCmd, lintConfigCmd},
}

type LintOpenapiFlags struct {
	SchemaPath            string `json:"schema"`
	Header                string `json:"header"`
	Token                 string `json:"token"`
	MaxValidationErrors   int    `json:"max-validation-errors"`
	MaxValidationWarnings int    `json:"max-validation-warnings"`
	Ruleset               string `json:"ruleset"`
}

var lintOpenapiCmd = model.ExecutableCommand[LintOpenapiFlags]{
	Usage:          "openapi",
	Short:          "Lint an OpenAPI document",
	Long:           `Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.`,
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
			Name:        "dir",
			Shorthand:   "d",
			Description: "path to the directory containing the Speakeasy configuration file",
			Required:    true,
		},
	},
}

func lintOpenapi(ctx context.Context, flags LintOpenapiFlags) error {
	// no authentication required for validating specs

	limits := validation.OutputLimits{
		MaxWarns:  flags.MaxValidationWarnings,
		MaxErrors: flags.MaxValidationErrors,
	}

	if err := validation.ValidateOpenAPI(ctx, flags.SchemaPath, flags.Header, flags.Token, &limits, "", ""); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := "speakeasy api register-schema --schema=" + flags.SchemaPath
	msg := fmt.Sprintf("\nYou can upload your schema to Speakeasy using the following command:\n%s", uploadCommand)
	log.From(ctx).Info(msg)

	return nil
}

func lintOpenapiInteractive(ctx context.Context, flags LintOpenapiFlags) error {
	limits := validation.OutputLimits{
		MaxWarns:  flags.MaxValidationWarnings,
		MaxErrors: flags.MaxValidationErrors,
	}

	if err := validation.ValidateWithInteractivity(ctx, flags.SchemaPath, flags.Header, flags.Token, &limits, "", ""); err != nil {
		return err
	}

	return nil
}

func lintConfig(ctx context.Context, flags lintConfigFlags) error {
	// no authentication required for validating configs

	if err := sdkgen.ValidateConfig(ctx, flags.Dir); err != nil {
		rootCmd.SilenceUsage = true

		return fmt.Errorf("%s", err)
	}

	log.From(ctx).Success("Config valid ✓")

	return nil
}
