package lint

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/arazzo"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/sdkgen"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"go.uber.org/zap"
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

	res, err := validation.ValidateOpenAPI(ctx, "", flags.SchemaPath, flags.Header, flags.Token, &limits, flags.Ruleset, wd, false, false, "")
	if err != nil {
		return err
	}

	// Run diagnostic dry run to surface generator warnings
	if err := runAndDisplayDiagnostics(ctx, flags.SchemaPath, res); err != nil {
		// Don't fail the command if diagnostics fail, just log the error
		log.From(ctx).Warnf("Failed to run diagnostics: %s", err.Error())
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

	logger := log.From(ctx)
	logger.Info("Linting OpenAPI document...\n")

	// Get schema contents
	isRemote, schema, err := openapi.GetSchemaContents(ctx, flags.SchemaPath, flags.Header, flags.Token)
	if err != nil {
		return fmt.Errorf("failed to get document contents: %w", err)
	}

	// Use the core Validate function to get results without displaying them
	res, err := validation.Validate(ctx, logger, schema, flags.SchemaPath, &limits, isRemote, flags.Ruleset, wd, false, false, "")
	if err != nil {
		return err
	}

	// Display all results in organized tabs
	if err := displayAllResultsInTabs(ctx, flags.SchemaPath, schema, res); err != nil {
		// Don't fail the command if diagnostics fail, just log the error
		log.From(ctx).Warnf("Failed to display results: %s", err.Error())
	}

	return nil
}

func lintConfig(ctx context.Context, flags lintConfigFlags) error {
	// To support the old version of this command, check if there is no workflow.yaml. If there isn't, run the old version
	wf, _, err := utils.GetWorkflowAndDir()
	if wf == nil || err != nil {
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

// runAndDisplayDiagnostics runs diagnostics on the schema and displays them in non-interactive mode
func runAndDisplayDiagnostics(ctx context.Context, schemaPath string, validationResult *validation.ValidationResult) error {
	logger := log.From(ctx)

	// Skip diagnostics if there were validation errors
	if validationResult != nil && len(validationResult.Errors) > 0 {
		return nil
	}

	logger.Info("Running SDK generation diagnostics...\n")

	// First, collect diagnostic suggestions about the OpenAPI structure
	diagnosis, err := suggest.Diagnose(ctx, schemaPath)
	if err != nil {
		return fmt.Errorf("failed to run diagnostics: %w", err)
	}

	// Try to get workflow file to run target-specific dry-run generations
	wf, projectDir, _ := utils.GetWorkflowAndDir()
	targetWarnings := make(map[string][]error)

	if wf != nil && len(wf.Targets) > 0 {
		// Run a dry-run generation for each target to collect target-specific warnings
		for targetName, target := range wf.Targets {
			warnings, err := runDryRunGeneration(ctx, schemaPath, target.Target, projectDir)
			if err == nil && len(warnings) > 0 {
				// Filter out warnings we've already seen
				newWarnings := []error{}
				for _, w := range warnings {
					isNew := true
					if validationResult != nil {
						for _, existingW := range validationResult.Warnings {
							if w.Error() == existingW.Error() {
								isNew = false
								break
							}
						}
					}
					if isNew {
						newWarnings = append(newWarnings, w)
					}
				}
				if len(newWarnings) > 0 {
					targetWarnings[targetName] = newWarnings
				}
			}
		}
	}

	// Check if we have any diagnostics or warnings to display
	totalDiagnostics := 0
	for _, diagnostics := range diagnosis {
		totalDiagnostics += len(diagnostics)
	}
	totalWarnings := 0
	for _, warnings := range targetWarnings {
		totalWarnings += len(warnings)
	}

	if totalDiagnostics == 0 && totalWarnings == 0 {
		logger.Successf("No SDK generation warnings found ✓\n")
		return nil
	}

	// Display diagnostics by type
	if totalDiagnostics > 0 {
		logger.Warnf("\nSDK Generation Suggestions: %d potential improvements found\n", totalDiagnostics)
		for diagType, diagnostics := range diagnosis {
			if len(diagnostics) == 0 {
				continue
			}

			logger.PrintfStyled(styles.HeavilyEmphasized, "\n%s (%d):", diagType, len(diagnostics))
			for _, d := range diagnostics {
				if len(d.SchemaPath) > 0 {
					logger.Printf("  • %s - %s", styles.Dimmed.Render(strings.Join(d.SchemaPath, " > ")), d.Message)
				} else {
					logger.Printf("  • %s", d.Message)
				}
			}
		}
	}

	// Display target-specific warnings
	if totalWarnings > 0 {
		logger.Warnf("\nSDK Generation Warnings (target-specific): %d warnings found\n", totalWarnings)
		prefixedLogger := logger.WithFormatter(log.PrefixedFormatter)
		for targetName, warnings := range targetWarnings {
			logger.PrintfStyled(styles.HeavilyEmphasized, "\n%s:", targetName)
			for _, w := range warnings {
				prefixedLogger.Warn("", zap.Error(w))
			}
		}
	}

	logger.PrintfStyled(styles.Info, "\nℹ Get automatic fixes in the Studio with %s\n", styles.HeavilyEmphasized.Render("speakeasy run --watch"))

	return nil
}

// displayAllResultsInTabs displays validation results, diagnostics, and generation warnings in organized tabs
func displayAllResultsInTabs(ctx context.Context, schemaPath string, schema []byte, validationResult *validation.ValidationResult) error {
	logger := log.From(ctx)

	var tabs []interactivity.Tab

	// Tab 1: Lint results - Errors
	errorCount := 0
	warningCount := 0
	hintCount := 0
	var errors []error
	var warnings []error
	var hints []error

	if validationResult != nil {
		errorCount = len(validationResult.Errors)
		warningCount = len(validationResult.Warnings)
		hintCount = len(validationResult.Infos)
		errors = validationResult.Errors
		warnings = validationResult.Warnings
		hints = validationResult.Infos
	}

	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Errors (%d)", errorCount),
		Content:     errorsToTabContents(schema, errors),
		TitleColor:  styles.Colors.Red,
		BorderColor: styles.Colors.Red,
		Default:     errorCount > 0,
	})

	// Tab 2: Lint results - Warnings
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Warnings (%d)", warningCount),
		Content:     errorsToTabContents(schema, warnings),
		TitleColor:  styles.Colors.Yellow,
		BorderColor: styles.Colors.Yellow,
		Default:     errorCount == 0 && warningCount > 0,
	})

	// Tab 3: Lint results - Hints
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Hints (%d)", hintCount),
		Content:     errorsToTabContents(schema, hints),
		TitleColor:  styles.Colors.Blue,
		BorderColor: styles.Colors.Blue,
		Default:     errorCount == 0 && warningCount == 0 && hintCount > 0,
	})

	// Skip diagnostics if there were validation errors
	if validationResult != nil && len(validationResult.Errors) > 0 {
		interactivity.RunTabs(tabs)
		return nil
	}

	logger.Info("Running SDK generation diagnostics...\n")

	// Tab 4: Suggestions (diagnostics)
	diagnosis, err := suggest.Diagnose(ctx, schemaPath)
	if err != nil {
		return fmt.Errorf("failed to run diagnostics: %w", err)
	}

	totalDiagnostics := 0
	for _, diagnostics := range diagnosis {
		totalDiagnostics += len(diagnostics)
	}

	suggestionsTitle := fmt.Sprintf("Suggestions (%d)", totalDiagnostics)
	if totalDiagnostics == 0 {
		suggestionsTitle = "Suggestions ✓"
	}

	tabs = append(tabs, interactivity.Tab{
		Title:       suggestionsTitle,
		Content:     diagnosisToTabContents(diagnosis),
		TitleColor:  styles.Colors.Blue,
		BorderColor: styles.Colors.Blue,
		Default:     false,
	})

	// Tabs 5+: Generation Warnings per target
	wf, projectDir, _ := utils.GetWorkflowAndDir()

	if wf != nil && len(wf.Targets) > 0 {
		// Sort target names for consistent ordering
		targetNames := slices.Collect(maps.Keys(wf.Targets))
		slices.Sort(targetNames)

		for _, targetName := range targetNames {
			target := wf.Targets[targetName]
			warnings, err := runDryRunGeneration(ctx, schemaPath, target.Target, projectDir)
			if err != nil {
				// If we can't run generation for this target, skip it
				continue
			}

			warningCount := len(warnings)
			warningTitle := fmt.Sprintf("Generation Warnings (%s)", targetName)

			if warningCount > 0 {
				warningTitle = fmt.Sprintf("Generation Warnings (%s) - %d", targetName, warningCount)
			}

			warningColor := styles.Colors.Green
			if warningCount > 0 {
				warningColor = styles.Colors.Yellow
			}

			tabs = append(tabs, interactivity.Tab{
				Title:       warningTitle,
				Content:     warningsToTabContents(warnings),
				TitleColor:  warningColor,
				BorderColor: warningColor,
				Default:     false,
			})
		}
	}

	interactivity.RunTabs(tabs)

	logger.PrintfStyled(styles.Info, "\nℹ Get automatic fixes in the Studio with %s\n", styles.HeavilyEmphasized.Render("speakeasy run --watch"))

	return nil
}

// diagnosisToTabContents converts diagnosis results to tab contents for interactive display
func diagnosisToTabContents(diagnosis suggestions.Diagnosis) []interactivity.InspectableContent {
	var contents []interactivity.InspectableContent

	for diagType, diagnostics := range diagnosis {
		if len(diagnostics) == 0 {
			continue
		}

		// Create the summary for this category
		summary := fmt.Sprintf("%s (%d)", diagType, len(diagnostics))

		// Create the detailed view with all diagnostics in this category
		var details strings.Builder
		for _, d := range diagnostics {
			if len(d.SchemaPath) > 0 {
				details.WriteString(fmt.Sprintf("• %s - %s\n", styles.Dimmed.Render(strings.Join(d.SchemaPath, " > ")), d.Message))
			} else {
				details.WriteString(fmt.Sprintf("• %s\n", d.Message))
			}
		}

		detailedView := strings.TrimSpace(details.String())
		content := interactivity.InspectableContent{
			Summary:      summary,
			DetailedView: &detailedView,
		}
		contents = append(contents, content)
	}

	if len(contents) == 0 {
		s := styles.Emphasized.Render("No suggestions found!")
		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: nil,
		}
		contents = append(contents, content)
	}

	return contents
}

// errorsToTabContents converts errors to tab contents for interactive display
func errorsToTabContents(schema []byte, errs []error) []interactivity.InspectableContent {
	var contents []interactivity.InspectableContent

	lines := strings.Split(string(schema), "\n")

	// Truncate very long lines
	for i, line := range lines {
		if len(line) > 1000 {
			lines[i] = line[:1000] + "..."
		}
	}

	for _, err := range errs {
		s := fmt.Sprintf("• %s", err.Error())

		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: nil,
		}

		contents = append(contents, content)
	}

	if len(errs) == 0 {
		s := styles.Emphasized.Render("No issues found!")
		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: nil,
		}
		contents = append(contents, content)
	}

	return contents
}

// warningsToTabContents converts warnings to tab contents for interactive display
func warningsToTabContents(warnings []error) []interactivity.InspectableContent {
	var contents []interactivity.InspectableContent

	for _, w := range warnings {
		content := interactivity.InspectableContent{
			Summary:      fmt.Sprintf("• %s", w.Error()),
			DetailedView: nil,
		}
		contents = append(contents, content)
	}

	if len(warnings) == 0 {
		s := styles.Emphasized.Render("No warnings found!")
		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: nil,
		}
		contents = append(contents, content)
	}

	return contents
}

// runDryRunGeneration runs a dry-run SDK generation for the specified target and returns warnings
func runDryRunGeneration(ctx context.Context, schemaPath, targetLanguage, workingDir string) ([]error, error) {
	// Load the OpenAPI schema
	isRemote, schema, err := openapi.GetSchemaContents(ctx, schemaPath, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get schema contents: %w", err)
	}

	// Create a logger that discards output (we only want warnings)
	silentLogger := log.From(ctx).WithWriter(io.Discard).WithFormatter(log.PrefixedFormatter)

	// Create generator with dry-run options
	opts := []generate.GeneratorOptions{
		generate.WithDontWrite(),
		generate.WithLogger(silentLogger),
		generate.WithRunLocation("cli"),
	}

	g, err := generate.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	// Dry-run the generation
	errs := g.Generate(ctx, schema, schemaPath, targetLanguage, workingDir, isRemote, false)
	if len(errs) > 0 {
		// Generation had errors, but we still want to collect warnings
		// We'll ignore generation errors for lint purposes
		// TODO: do we want to also show the errors?
	}

	// Collect warnings from the generator
	warnings := g.GetWarnings()

	return warnings, nil
}
