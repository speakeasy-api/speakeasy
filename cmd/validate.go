package cmd

import (
	"context"
	"fmt"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/maps"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/validation"
)

var validateCmd = &model.CommandGroup{
	Usage:          "validate",
	Short:          "Validate OpenAPI documents + more",
	Long:           `The "validate" command provides a set of commands for validating OpenAPI docs and more.`,
	InteractiveMsg: "What do you want to validate?",
	Commands:       []model.Command{validateOpenapiCmd, validateConfigCmd},
}

type ValidateOpenapiFlags struct {
	SchemaPath            string `json:"schema"`
	Header                string `json:"header"`
	Token                 string `json:"token"`
	OutputHints           bool   `json:"output-hints"`
	MaxValidationErrors   int    `json:"max-validation-errors"`
	MaxValidationWarnings int    `json:"max-validation-warnings"`
}

var validateOpenapiCmd = model.ExecutableCommand[ValidateOpenapiFlags]{
	Usage:          "openapi",
	Short:          "Validate an OpenAPI document",
	Long:           `Validates an OpenAPI document is valid and conforms to the Speakeasy OpenAPI specification.`,
	Run:            validateOpenapi,
	RunInteractive: validateOpenapiInteractive,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:                       "schema",
			Shorthand:                  "s",
			Description:                "local filepath or URL for the OpenAPI schema",
			Required:                   true,
			AutocompleteFileExtensions: charm_internal.OpenAPIFileExtensions,
		},
		flag.BooleanFlag{
			Name:         "output-hints",
			Shorthand:    "o",
			Description:  "output validation hints in addition to warnings/errors",
			DefaultValue: false,
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
	},
}

type validateConfigFlags struct {
	Dir string `json:"dir"`
}

var validateConfigCmd = &model.ExecutableCommand[validateConfigFlags]{
	Usage: "config",
	Short: "Validate a Speakeasy configuration file",
	Long:  `Validates a Speakeasy configuration file for SDK generation.`,
	Run:   validateConfig,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "path to the directory containing the Speakeasy configuration file",
			DefaultValue: ".",
		},
	},
}

func validateOpenapi(ctx context.Context, flags ValidateOpenapiFlags) error {
	// no authentication required for validating specs

	limits := validation.OutputLimits{
		OutputHints: flags.OutputHints,
		MaxWarns:    flags.MaxValidationWarnings,
		MaxErrors:   flags.MaxValidationErrors,
	}

	if err := validation.ValidateOpenAPI(ctx, flags.SchemaPath, flags.Header, flags.Token, &limits); err != nil {
		rootCmd.SilenceUsage = true

		return err
	}

	uploadCommand := "speakeasy api register-schema --schema=" + flags.SchemaPath
	msg := fmt.Sprintf("\nYou can upload your schema to Speakeasy using the following command:\n%s", uploadCommand)
	log.From(ctx).Info(msg)

	return nil
}

func validateOpenapiInteractive(ctx context.Context, flags ValidateOpenapiFlags) error {
	limits := validation.OutputLimits{
		OutputHints: flags.OutputHints,
		MaxWarns:    flags.MaxValidationWarnings,
		MaxErrors:   flags.MaxValidationErrors,
	}

	if err := validation.ValidateWithInteractivity(ctx, flags.SchemaPath, flags.Header, flags.Token, &limits); err != nil {
		return err
	}

	return nil
}

func validateConfig(ctx context.Context, flags validateConfigFlags) error {
	// To support the old version of this command, check if there is no workflow.yaml. If there isn't, run the old version
	wf, _, err := utils.GetWorkflowAndDir()
	if wf == nil {
		return sdkgen.ValidateConfig(ctx, flags.Dir)
	}

	// Below is the workflow file based version of this command

	targetToConfig, err := validation.GetAndValidateConfigs(ctx)
	if err != nil {
		return err
	}

	langs := strings.Join(maps.Keys(targetToConfig), ", ")

	msg := styles.RenderSuccessMessage(
		"SDK generation configuration is valid ✓",
		"Validated targets: "+langs,
	)

	log.From(ctx).Println(msg)

	return nil
}
