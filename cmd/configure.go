package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pkg/errors"
	spkErrors "github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/testcmd"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/prompts"
)

const (
	repositorySecretPath    = "Settings > Secrets & Variables > Actions"
	actionsPath             = "Actions > Generate"
	actionsSettingsPath     = "Settings > Actions > General"
	githubSetupDocs         = "https://www.speakeasy.com/docs/advanced-setup/github-setup"
	appInstallURL           = "https://github.com/apps/speakeasy-github"
	ErrWorkflowFileNotFound = spkErrors.Error("workflow.yaml file not found")
	testingSetupDocs        = "https://go.speakeasy.com/setup-tests"
	testCheckDocs           = "https://go.speakeasy.com/test-checks"
)

const configureLong = `# Configure

Configure your Speakeasy workflow file.

[Workflows](https://www.speakeasy.com/docs/workflow-file-reference)

[GitHub Setup](https://www.speakeasy.com/docs/publish-sdks/github-setup)

[Publishing](https://www.speakeasy.com/docs/publish-sdks/publish-sdks)

[Testing](https://www.speakeasy.com/docs/customize-testing/bootstrapping-test-generation)

`

var configureCmd = &model.CommandGroup{
	Usage:          "configure",
	Short:          "Configure your Speakeasy SDK Setup.",
	Long:           utils.RenderMarkdown(configureLong),
	InteractiveMsg: "What do you want to configure?",
	Commands:       []model.Command{configureSourcesCmd, configureTargetCmd, configureGithubCmd, configurePublishingCmd, configureTestingCmd, configureLocalWorkflowCmd, configureGenerationCmd},
}

var configureGenerationCmd = &model.CommandGroup{
	Usage:          "generation",
	Short:          "Configure and inspect generation settings.",
	Long:           "Commands for inspecting and managing SDK generation configuration.",
	InteractiveMsg: "What would you like to do?",
	Commands:       []model.Command{configureGenerationCheckCmd},
}

type ConfigureSourcesFlags struct {
	ID             string `json:"id"`
	New            bool   `json:"new"`
	Location       string `json:"location"`
	SourceName     string `json:"source-name"`
	AuthHeader     string `json:"auth-header"`
	OutputPath     string `json:"output"`
	NonInteractive bool   `json:"non-interactive"`
}

var configureSourcesCmd = &model.ExecutableCommand[ConfigureSourcesFlags]{
	Usage:        "sources",
	Short:        "Configure new or existing sources.",
	Long:         "Guided prompts to configure a new or existing source in your speakeasy workflow. When --location and --source-name are provided, runs in non-interactive mode suitable for CI/CD.",
	Run:          configureSources,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "id",
			Shorthand:   "i",
			Description: "the name of an existing source to configure",
		},
		flag.BooleanFlag{
			Name:        "new",
			Shorthand:   "n",
			Description: "configure a new source",
		},
		flag.StringFlag{
			Name:        "location",
			Shorthand:   "l",
			Description: "location of the OpenAPI document (local file path or URL); enables non-interactive mode when combined with --source-name",
		},
		flag.StringFlag{
			Name:        "source-name",
			Shorthand:   "s",
			Description: "name for the source; enables non-interactive mode when combined with --location",
		},
		flag.StringFlag{
			Name:        "auth-header",
			Description: "authentication header name for remote documents (value from $OPENAPI_DOC_AUTH_TOKEN)",
		},
		flag.StringFlag{
			Name:        "output",
			Shorthand:   "o",
			Description: "output path for the compiled source document",
		},
		flag.BooleanFlag{
			Name:        "non-interactive",
			Description: "run in non-interactive mode; requires --location and --source-name",
		},
	},
}

type ConfigureTargetFlags struct {
	ID             string `json:"id"`
	New            bool   `json:"new"`
	TargetType     string `json:"target-type"`
	SourceID       string `json:"source"`
	TargetName     string `json:"target-name"`
	SDKClassName   string `json:"sdk-class-name"`
	PackageName    string `json:"package-name"`
	BaseServerURL  string `json:"base-server-url"`
	OutputDir      string `json:"output"`
	NonInteractive bool   `json:"non-interactive"`
}

var configureTargetCmd = &model.ExecutableCommand[ConfigureTargetFlags]{
	Usage:        "targets",
	Short:        "Configure new or existing targets.",
	Long:         "Guided prompts to configure a new or existing target in your speakeasy workflow. When --target-type and --source are provided, runs in non-interactive mode suitable for CI/CD.",
	Run:          configureTarget,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "id",
			Shorthand:   "i",
			Description: "the name of an existing target to configure",
		},
		flag.BooleanFlag{
			Name:        "new",
			Shorthand:   "n",
			Description: "configure a new target",
		},
		flag.StringFlag{
			Name:        "target-type",
			Shorthand:   "t",
			Description: "target language/type: typescript, python, go, java, csharp, php, ruby, terraform, mcp-typescript; enables non-interactive mode",
		},
		flag.StringFlag{
			Name:        "source",
			Shorthand:   "s",
			Description: "name of the source to generate from; enables non-interactive mode when combined with --target-type",
		},
		flag.StringFlag{
			Name:        "target-name",
			Description: "name for the target (defaults to target-type if not specified)",
		},
		flag.StringFlag{
			Name:        "sdk-class-name",
			Description: "SDK class name (PascalCase, e.g., MyCompanySDK)",
		},
		flag.StringFlag{
			Name:        "package-name",
			Description: "package name for the generated SDK",
		},
		flag.StringFlag{
			Name:        "base-server-url",
			Description: "base server URL for the SDK",
		},
		flag.StringFlag{
			Name:        "output",
			Shorthand:   "o",
			Description: "output directory for the generated target",
		},
		flag.BooleanFlag{
			Name:        "non-interactive",
			Description: "run in non-interactive mode; requires --target-type and --source",
		},
	},
}

type ConfigureTestsFlags struct {
	Rebuild           *string `json:"rebuild"`
	WorkflowDirectory string  `json:"workflow-directory"`
}

type ConfigureGithubFlags struct {
	WorkflowDirectory string `json:"workflow-directory"`
}

type ConfigurePublishingFlags struct {
	WorkflowDirectory     string `json:"workflow-directory"`
	PyPITrustedPublishing bool   `json:"pypi-trusted-publishing"`
}

var configureGithubCmd = &model.ExecutableCommand[ConfigureGithubFlags]{
	Usage: "github",
	Short: "Configure Speakeasy for github.",
	Long:  "Configure your Speakeasy workflow to generate and publish from your github repo.",
	Run:   configureGithub,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "workflow-directory",
			Shorthand:   "d",
			Description: "directory of speakeasy workflow file",
		},
	},
	RequiresAuth: true,
}

var configurePublishingCmd = &model.ExecutableCommand[ConfigurePublishingFlags]{
	Usage: "publishing",
	Short: "Configure Speakeasy for publishing.",
	Long:  "Configure your Speakeasy workflow to publish to package managers from your github repo.",
	Run:   configurePublishing,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "workflow-directory",
			Shorthand:   "d",
			Description: "directory of speakeasy workflow file",
		},
		flag.BooleanFlag{
			Name:        "pypi-trusted-publishing",
			Description: "use PyPI trusted publishing instead of API tokens for Python targets",
		},
	},
	RequiresAuth: true,
}

var configureTestingCmd = &model.ExecutableCommand[ConfigureTestsFlags]{
	Usage: "tests",
	Short: "Configure Speakeasy SDK tests.",
	Long:  "Configure your Speakeasy workflow to generate and run SDK tests..",
	Run:   configureTesting,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "workflow-directory",
			Shorthand:   "d",
			Description: "directory of speakeasy workflow file",
		},
		flag.StringFlagWithOptionalValue{
			Name:         "rebuild",
			Description:  "clears out all existing tests and regenerates them from scratch or if operations are specified will rebuild the tests for those operations (multiple operations can be specified as a single comma separated value)",
			DefaultValue: "*",
		},
	},
	RequiresAuth: true,
}

type ConfigureLocalWorkflowFlags struct {
	WorkflowDirectory string `json:"workflow-directory"`
}

var configureLocalWorkflowCmd = &model.ExecutableCommand[ConfigureLocalWorkflowFlags]{
	Usage: "local-workflow",
	Short: "Create a local workflow configuration file.",
	Long:  "Copies workflow.yaml to workflow.local.yaml with all settings commented out for local overrides.\n\nThis file is intended for local development only and should be added to .gitignore.\nIf you need a shared workflow configuration (e.g. generating multiple API versions in CI),\ndefine those targets directly in workflow.yaml instead.",
	Run:   configureLocalWorkflow,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "workflow-directory",
			Shorthand:   "d",
			Description: "directory of speakeasy workflow file",
		},
	},
}

func configureSources(ctx context.Context, flags ConfigureSourcesFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	var workflowFile *workflow.Workflow
	if workflowFile, _, _ = workflow.Load(workingDir); workflowFile == nil {
		workflowFile = &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		}
	}

	// Non-interactive mode: when both --location and --source-name are provided
	if flags.Location != "" && flags.SourceName != "" {
		return configureSourcesNonInteractive(ctx, workingDir, workflowFile, flags)
	}

	// If --non-interactive flag is set but required args are missing, return a helpful error
	if flags.NonInteractive {
		if err := checkNonInteractiveSourcesFlags(flags); err != nil {
			return err
		}
	}

	var existingSourceName string
	var existingSource *workflow.Source
	if source, ok := workflowFile.Sources[flags.ID]; ok {
		existingSourceName = flags.ID
		existingSource = &source
	}

	var sourceOptions []huh.Option[string]
	for sourceName := range workflowFile.Sources {
		sourceOptions = append(sourceOptions, huh.NewOption(charm.FormatEditOption(sourceName), sourceName))
	}
	sourceOptions = append(sourceOptions, huh.NewOption(charm.FormatNewOption("New Source"), "new source"))

	if !flags.New && existingSource == nil {
		prompt := charm.NewSelectPrompt("What source would you like to configure?", "You may choose an existing source or create a new source.", sourceOptions, &existingSourceName)
		if _, err := charm.NewForm(
			huh.NewForm(prompt),
			charm.WithTitle("Let's configure a source for your workflow."),
		).ExecuteForm(); err != nil {
			return err
		}
		if existingSourceName == "new source" {
			existingSourceName = ""
		} else {
			if source, ok := workflowFile.Sources[existingSourceName]; ok {
				existingSource = &source
			}
		}
	}

	if existingSource != nil {
		source, err := prompts.AddToSource(existingSourceName, existingSource)
		if err != nil {
			return errors.Wrapf(err, "failed to add to source %s", existingSourceName)
		}
		workflowFile.Sources[existingSourceName] = *source
	} else {
		newName, source, err := prompts.PromptForNewSource(workflowFile)
		if err != nil {
			return errors.Wrap(err, "failed to prompt for a new source")
		}

		workflowFile.Sources[newName] = *source
		existingSourceName = newName
	}

	if err := workflowFile.Validate(generate.GetSupportedTargetNames()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		err = os.MkdirAll(".speakeasy", 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.Colors.Green).Padding(0, 1)
	successMsg := fmt.Sprintf("Successfully Configured the Source %s 🎉", existingSourceName)
	if workflowFile.Targets != nil || len(workflowFile.Targets) > 0 {
		successMsg += "\n\nExecute speakeasy run to regenerate your SDK!"
	}
	success := styles.Success.Render(successMsg)
	logger := log.From(ctx)
	logger.PrintfStyled(boxStyle, "%s", success)

	return nil
}

// configureSourcesNonInteractive handles source configuration without interactive prompts.
// This is used when --location and --source-name flags are both provided.
func configureSourcesNonInteractive(ctx context.Context, workingDir string, workflowFile *workflow.Workflow, flags ConfigureSourcesFlags) error {
	logger := log.From(ctx)

	// Validate source name doesn't already exist
	if _, ok := workflowFile.Sources[flags.SourceName]; ok {
		return fmt.Errorf("a source with the name %q already exists", flags.SourceName)
	}

	// Validate source name format
	if strings.Contains(flags.SourceName, " ") {
		return fmt.Errorf("source name must not contain spaces")
	}

	// Build the source document
	document := workflow.Document{
		Location: workflow.LocationString(flags.Location),
	}

	// Add authentication if provided
	if flags.AuthHeader != "" {
		document.Auth = &workflow.Auth{
			Header: flags.AuthHeader,
			Secret: "$openapi_doc_auth_token",
		}
	}

	// Build the source
	source := workflow.Source{
		Inputs: []workflow.Document{document},
	}

	// Set output path if provided
	if flags.OutputPath != "" {
		source.Output = &flags.OutputPath
	}

	// Validate the source
	if err := source.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate source configuration")
	}

	// Add source to workflow
	workflowFile.Sources[flags.SourceName] = source

	// Validate the workflow
	if err := workflowFile.Validate(generate.GetSupportedTargetNames()); err != nil {
		return errors.Wrap(err, "failed to validate workflow file")
	}

	// Ensure .speakeasy directory exists
	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		if err := os.MkdirAll(".speakeasy", 0o755); err != nil {
			return err
		}
	}

	// Save the workflow
	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrap(err, "failed to save workflow file")
	}

	// Print success message
	logger.Printf("Successfully configured source %q with location %q\n", flags.SourceName, flags.Location)
	if flags.OutputPath != "" {
		logger.Printf("  Output path: %s\n", flags.OutputPath)
	}
	if flags.AuthHeader != "" {
		logger.Printf("  Auth header: %s (value from $OPENAPI_DOC_AUTH_TOKEN)\n", flags.AuthHeader)
	}

	return nil
}

// checkNonInteractiveSourcesFlags validates that required flags are provided for non-interactive mode.
// Returns an error listing missing flags if any are not provided.
func checkNonInteractiveSourcesFlags(flags ConfigureSourcesFlags) error {
	var missing []string
	if flags.Location == "" {
		missing = append(missing, "--location")
	}
	if flags.SourceName == "" {
		missing = append(missing, "--source-name")
	}
	if len(missing) > 0 {
		return fmt.Errorf("non-interactive mode requires the following flags: %s", strings.Join(missing, ", "))
	}
	return nil
}

// checkNonInteractiveTargetFlags validates that required flags are provided for non-interactive mode.
// Returns an error listing missing flags if any are not provided.
func checkNonInteractiveTargetFlags(flags ConfigureTargetFlags) error {
	var missing []string
	if flags.TargetType == "" {
		missing = append(missing, "--target-type")
	}
	if flags.SourceID == "" {
		missing = append(missing, "--source")
	}
	if len(missing) > 0 {
		return fmt.Errorf("non-interactive mode requires the following flags: %s", strings.Join(missing, ", "))
	}
	return nil
}

func configureTarget(ctx context.Context, flags ConfigureTargetFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	existingSDK := prompts.HasExistingGeneration(workingDir)

	var workflowFile *workflow.Workflow
	if workflowFile, _, err = workflow.Load(workingDir); err != nil || workflowFile == nil || len(workflowFile.Sources) == 0 {
		suggestion := "speakeasy quickstart"
		if existingSDK {
			suggestion = "speakeasy configure sources"
		}
		return errors.New(fmt.Sprintf("you must have a source to configure a target try, %s", suggestion))
	}

	if workflowFile.Targets == nil {
		workflowFile.Targets = make(map[string]workflow.Target)
	}

	// Non-interactive mode: when --target-type and --source are provided
	if flags.TargetType != "" && flags.SourceID != "" {
		return configureTargetNonInteractive(ctx, workingDir, workflowFile, flags)
	}

	// If --non-interactive flag is set but required args are missing, return a helpful error
	if flags.NonInteractive {
		if err := checkNonInteractiveTargetFlags(flags); err != nil {
			return err
		}
	}

	existingTarget := ""
	if _, ok := workflowFile.Targets[flags.ID]; ok {
		existingTarget = flags.ID
	}

	newTarget := flags.New

	var targetOptions []huh.Option[string]
	var existingTargets []string
	if len(workflowFile.Targets) > 0 {
		for targetName := range workflowFile.Targets {
			existingTargets = append(existingTargets, targetName)
			targetOptions = append(targetOptions, huh.NewOption(charm.FormatEditOption(targetName), targetName))
		}
		targetOptions = append(targetOptions, huh.NewOption(charm.FormatNewOption("New Target"), "new target"))
	} else {
		// To support legacy SDK configurations configure will detect an existing target setup in the current root directory
		if existingSDK {
			existingTargets, targetOptions = handleLegacySDKTarget(workingDir, workflowFile)
		} else {
			newTarget = true
		}
	}

	if !newTarget && existingTarget == "" {
		prompt := charm.NewSelectPrompt("What target would you like to configure?", "You may choose an existing target or create a new target.", targetOptions, &existingTarget)
		if _, err := charm.NewForm(huh.NewForm(prompt),
			charm.WithTitle("Let's configure a target for your workflow.")).
			ExecuteForm(); err != nil {
			return err
		}
		if existingTarget == "new target" {
			existingTarget = ""
		}
	}

	var targetName string
	var target *workflow.Target
	var targetConfig *config.Configuration
	if existingTarget == "" {
		if len(workflowFile.Targets) > 0 {
			// If a second target is added to an existing workflow file you must change the outdir of either target cannot be the root dir.
			if err := prompts.PromptForOutDirMigration(workflowFile, existingTargets); err != nil {
				return err
			}
		}

		targetName, target, err = prompts.PromptForNewTarget(workflowFile, "", "", "")
		if err != nil {
			return err
		}

		workflowFile.Targets[targetName] = *target

		targetConfig, err = prompts.PromptForTargetConfig(targetName, workflowFile, target, nil, nil)
		if err != nil {
			return err
		}
	} else {
		targetName, target, err = prompts.PromptForExistingTarget(workflowFile, existingTarget)
		if err != nil {
			return err
		}

		if targetName != existingTarget {
			delete(workflowFile.Targets, existingTarget)
		}

		workflowFile.Targets[targetName] = *target

		configDir := workingDir
		if target.Output != nil {
			configDir = *target.Output
		}

		var existingConfig *config.Configuration
		if cfg, err := config.Load(configDir); err == nil {
			existingConfig = cfg.Config
		}

		targetConfig, err = prompts.PromptForTargetConfig(targetName, workflowFile, target, existingConfig, nil)
		if err != nil {
			return err
		}
	}

	outDir := workingDir
	if target.Output != nil {
		outDir = *target.Output
	}

	if _, err := os.Stat(filepath.Join(outDir, ".speakeasy")); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(outDir, ".speakeasy"), 0o755)
		if err != nil {
			return err
		}
	}

	// If we are creating a new target we must make sure an empty gen.yaml file exists so SaveConfig writes in the correct place
	if existingTarget == "" {
		if _, err := os.Stat(filepath.Join(outDir, ".speakeasy/gen.yaml")); os.IsNotExist(err) {
			err = os.WriteFile(filepath.Join(outDir, ".speakeasy/gen.yaml"), []byte{}, 0o644)
			if err != nil {
				return err
			}
		}
	}

	if err := config.SaveConfig(outDir, targetConfig); err != nil {
		return errors.Wrapf(err, "failed to save config file for target %s", targetName)
	}

	if err := workflowFile.Validate(generate.GetSupportedTargetNames()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		err = os.MkdirAll(".speakeasy", 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	successMsg := fmt.Sprintf("Successfully Configured the Target %s 🎉", targetName)
	if len(workflowFile.Targets) > 0 {
		successMsg += "\n\nExecute speakeasy run to regenerate your SDK!"
	}

	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.Colors.Green).Padding(0, 1)
	success := styles.Success.Render(successMsg)
	logger := log.From(ctx)
	logger.PrintfStyled(boxStyle, "%s", success)

	return nil
}

// configureTargetNonInteractive handles target configuration without interactive prompts.
// This is used when --target-type and --source flags are both provided.
func configureTargetNonInteractive(ctx context.Context, workingDir string, workflowFile *workflow.Workflow, flags ConfigureTargetFlags) error {
	logger := log.From(ctx)

	// Validate target type is supported
	supportedTargets := generate.GetSupportedTargetNames()
	if !slices.Contains(supportedTargets, flags.TargetType) {
		return fmt.Errorf("unsupported target type %q; supported types: %s", flags.TargetType, strings.Join(supportedTargets, ", "))
	}

	// Validate source exists
	if _, ok := workflowFile.Sources[flags.SourceID]; !ok {
		var sourceNames []string
		for name := range workflowFile.Sources {
			sourceNames = append(sourceNames, name)
		}
		return fmt.Errorf("source %q not found; available sources: %s", flags.SourceID, strings.Join(sourceNames, ", "))
	}

	// Default target name to target type if not provided
	targetName := flags.TargetName
	if targetName == "" {
		targetName = flags.TargetType
	}

	// Validate target name doesn't already exist
	if _, ok := workflowFile.Targets[targetName]; ok {
		return fmt.Errorf("a target with the name %q already exists", targetName)
	}

	// Validate target name format
	if strings.Contains(targetName, " ") {
		return fmt.Errorf("target name must not contain spaces")
	}

	// Build the target
	target := workflow.Target{
		Target: flags.TargetType,
		Source: flags.SourceID,
	}

	// Set output directory if provided
	if flags.OutputDir != "" {
		target.Output = &flags.OutputDir
	}

	// Validate the target
	if err := target.Validate(supportedTargets, workflowFile.Sources); err != nil {
		return errors.Wrap(err, "failed to validate target configuration")
	}

	// Add target to workflow
	workflowFile.Targets[targetName] = target

	// Build config for target
	targetConfig, err := config.GetDefaultConfig(true, generate.GetLanguageConfigDefaults, map[string]bool{flags.TargetType: true})
	if err != nil {
		return errors.Wrapf(err, "failed to generate config for target %s", targetName)
	}

	// Set SDK class name if provided
	if flags.SDKClassName != "" {
		targetConfig.Generation.SDKClassName = flags.SDKClassName
	}

	// Set base server URL if provided
	if flags.BaseServerURL != "" {
		targetConfig.Generation.BaseServerURL = flags.BaseServerURL
	}

	// Set package name if provided
	if flags.PackageName != "" {
		if langConfig, ok := targetConfig.Languages[flags.TargetType]; ok {
			if langConfig.Cfg == nil {
				langConfig.Cfg = make(map[string]interface{})
			}
			// Different languages use different config keys for package name
			switch flags.TargetType {
			case "go":
				langConfig.Cfg["modulePath"] = flags.PackageName
			case "java":
				// For Java, split packageName into groupID and artifactID if it contains a colon
				if strings.Contains(flags.PackageName, ":") {
					parts := strings.SplitN(flags.PackageName, ":", 2)
					langConfig.Cfg["groupID"] = parts[0]
					langConfig.Cfg["artifactID"] = parts[1]
				} else {
					langConfig.Cfg["groupID"] = flags.PackageName
				}
			default:
				langConfig.Cfg["packageName"] = flags.PackageName
			}
			targetConfig.Languages[flags.TargetType] = langConfig
		}
	}

	// Determine output directory
	outDir := workingDir
	if target.Output != nil {
		outDir = *target.Output
	}

	// Ensure .speakeasy directory exists in output dir
	if _, err := os.Stat(filepath.Join(outDir, ".speakeasy")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(outDir, ".speakeasy"), 0o755); err != nil {
			return err
		}
	}

	// Create empty gen.yaml if it doesn't exist
	genYamlPath := filepath.Join(outDir, ".speakeasy/gen.yaml")
	if _, err := os.Stat(genYamlPath); os.IsNotExist(err) {
		if err := os.WriteFile(genYamlPath, []byte{}, 0o644); err != nil {
			return err
		}
	}

	// Save config
	if err := config.SaveConfig(outDir, targetConfig); err != nil {
		return errors.Wrapf(err, "failed to save config for target %s", targetName)
	}

	// Validate the workflow
	if err := workflowFile.Validate(supportedTargets); err != nil {
		return errors.Wrap(err, "failed to validate workflow file")
	}

	// Ensure .speakeasy directory exists in working dir
	if _, err := os.Stat(".speakeasy"); os.IsNotExist(err) {
		if err := os.MkdirAll(".speakeasy", 0o755); err != nil {
			return err
		}
	}

	// Save the workflow
	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrap(err, "failed to save workflow file")
	}

	// Print success message
	logger.Printf("Successfully configured target %q\n", targetName)
	logger.Printf("  Type: %s\n", flags.TargetType)
	logger.Printf("  Source: %s\n", flags.SourceID)
	if flags.OutputDir != "" {
		logger.Printf("  Output: %s\n", flags.OutputDir)
	}
	if flags.SDKClassName != "" {
		logger.Printf("  SDK Class Name: %s\n", flags.SDKClassName)
	}
	if flags.PackageName != "" {
		logger.Printf("  Package Name: %s\n", flags.PackageName)
	}
	if flags.BaseServerURL != "" {
		logger.Printf("  Base Server URL: %s\n", flags.BaseServerURL)
	}

	return nil
}

func configurePublishing(ctx context.Context, flags ConfigurePublishingFlags) error {
	logger := log.From(ctx)

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	actionWorkingDir := getActionWorkingDirectoryFromFlag(rootDir, flags.WorkflowDirectory)

	workflowFile, workflowFilePath, _ := workflow.Load(filepath.Join(rootDir, actionWorkingDir))
	if workflowFile == nil {
		return renderAndPrintWorkflowNotFound("publishing", logger)
	}

	var publishingOptions []huh.Option[string]
	for name, target := range workflowFile.Targets {
		if slices.Contains(prompts.SupportedPublishingTargets, target.Target) {
			publishingOptions = append(publishingOptions, huh.NewOption(fmt.Sprintf("%s [%s]", name, strings.ToUpper(target.Target)), name))
		}
	}

	var chosenTargets []string
	switch {
	case len(publishingOptions) == 0:
		logger.Println(styles.Info.Render("No existing SDK targets require package manager publishing configuration."))
		return nil
	case len(publishingOptions) == 1:
		chosenTargets = []string{publishingOptions[0].Value}
	default:
		chosenTargets, err = prompts.SelectPublishingTargets(publishingOptions, true)
		if err != nil {
			return err
		}
	}

	if len(chosenTargets) == 0 {
		logger.Println(styles.Info.Render("No targets selected. Exiting."))
		return nil
	}

	for _, name := range chosenTargets {
		target := workflowFile.Targets[name]
		modifiedTarget, err := prompts.ConfigurePublishing(&target, name, prompts.ConfigurePublishingOptions{
			PyPITrustedPublishing: flags.PyPITrustedPublishing,
		})
		if err != nil {
			return err
		}
		workflowFile.Targets[name] = *modifiedTarget
	}

	secrets := make(map[string]string)
	workflowPaths := make(map[string]targetWorkflowPaths)

	for _, name := range chosenTargets {
		generationWorkflow, targetWorkflowPaths, err := writePublishingFile(workflowFile, workflowFile.Targets[name], name, rootDir, actionWorkingDir)
		if err != nil {
			return err
		}
		for key, val := range generationWorkflow.Jobs.Generate.Secrets {
			secrets[key] = val
		}

		workflowPaths[name] = targetWorkflowPaths
	}

	if err := workflow.Save(filepath.Join(rootDir, actionWorkingDir), workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	var remoteURL string
	if repo := prompts.FindGithubRepository(rootDir); repo != nil {
		remoteURL = prompts.ParseGithubRemoteURL(repo)
	}

	secretPath := repositorySecretPath
	if remoteURL != "" {
		secretPath = fmt.Sprintf("%s/settings/secrets/actions", remoteURL)
	}

	actionPath := actionsPath
	if remoteURL != "" {
		actionPath = fmt.Sprintf("%s/actions", remoteURL)
	}

	status := []string{
		fmt.Sprintf("Speakeasy workflow written to - %s", workflowFilePath),
	}

	var publishPaths, generationWorkflowFilePaths []string
	for _, wfp := range workflowPaths {
		publishPaths = append(publishPaths, wfp.publishWorkflowPaths...)
		generationWorkflowFilePaths = append(generationWorkflowFilePaths, wfp.generationWorkflowPath)
	}
	if len(publishPaths) > 0 {
		status = append(status, "GitHub action (generate) written to:")
		for _, path := range generationWorkflowFilePaths {
			status = append(status, fmt.Sprintf("\t- %s", path))
		}
		status = append(status, "GitHub action (publish) written to:")
		for _, path := range publishPaths {
			status = append(status, fmt.Sprintf("\t- %s", path))
		}
	} else {
		status = append(status, "GitHub action (generate+publish) written to:")
		for _, path := range generationWorkflowFilePaths {
			status = append(status, fmt.Sprintf("\t- %s", path))
		}
	}

	agenda := []string{
		fmt.Sprintf("• On GitHub navigate to %s and set up the following repository secrets:", secretPath),
	}

	for key := range secrets {
		if key != config.GithubAccessToken {
			agenda = append(agenda, fmt.Sprintf("\t◦ Provide a secret with name %s", styles.MakeBold(strings.ToUpper(key))))
		}
	}

	agenda = append(agenda, fmt.Sprintf("• Push your repository to GitHub and navigate to %s to kick off your first publish!", actionPath))

	// Add instructions for NPM Trusted Publishing (typescript/mcp-typescript)
	npmTrustedPublishingConfigs := make(map[string]NPMTrustedPublishingConfig)
	for name, wfp := range workflowPaths {
		target := workflowFile.Targets[name]
		if target.Publishing != nil && target.Publishing.NPM != nil {
			var publishPath string
			switch len(wfp.publishWorkflowPaths) {
			case 0:
				// No publish path means generation and publishing are combined into a single workflow
				publishPath = wfp.generationWorkflowPath
			case 1:
				publishPath = wfp.publishWorkflowPaths[0]
			default:
				// For typescript/mcp-typescript, if the publish and generation workflow files
				// are distinct (pr mode), then we only expect a single publish path.
				return errors.Wrapf(err, "multiple publish workflow paths found for target %s", name)
			}

			// Get the packageName from the config file
			packageName := "<packageName>"
			outDir := ""
			if target.Output != nil {
				outDir = *target.Output
			}
			workflowDir := filepath.Join(rootDir, actionWorkingDir)
			configPath := filepath.Join(workflowDir, outDir)
			cfg, err := config.Load(configPath)
			if err == nil {
				if langCfg, ok := cfg.Config.Languages[target.Target]; ok {
					if pkgName, ok := langCfg.Cfg["packageName"].(string); ok {
						packageName = pkgName
					}
				}
			}

			npmTrustedPublishingConfigs[name] = NPMTrustedPublishingConfig{
				target:          target,
				workflowDir:     filepath.Join(rootDir, actionWorkingDir),
				actionPath:      actionPath,
				publishFileName: filepath.Base(publishPath),
				packageName:     packageName,
				remoteURL:       remoteURL,
			}
		}
	}
	agenda = append(agenda, getNPMTrustedPublishingInstructions(ctx, npmTrustedPublishingConfigs)...)

	// Add instructions for PyPI Trusted Publishing (python)
	pypiTrustedPublishingConfigs := make(map[string]PyPITrustedPublishingConfig)
	for name, wfp := range workflowPaths {
		target := workflowFile.Targets[name]
		if target.Publishing != nil && target.Publishing.PyPi != nil && target.Publishing.PyPi.UseTrustedPublishing != nil && *target.Publishing.PyPi.UseTrustedPublishing {
			var publishPath string
			switch len(wfp.publishWorkflowPaths) {
			case 0:
				// No publish path means generation and publishing are combined into a single workflow
				publishPath = wfp.generationWorkflowPath
			case 1:
				publishPath = wfp.publishWorkflowPaths[0]
			default:
				// For python, if the publish and generation workflow files
				// are distinct (pr mode), then we only expect a single publish path.
				return errors.Wrapf(err, "multiple publish workflow paths found for target %s", name)
			}

			// Get the packageName from the config file
			packageName := "<packageName>"
			outDir := ""
			if target.Output != nil {
				outDir = *target.Output
			}
			workflowDir := filepath.Join(rootDir, actionWorkingDir)
			configPath := filepath.Join(workflowDir, outDir)
			cfg, err := config.Load(configPath)
			if err == nil {
				if langCfg, ok := cfg.Config.Languages[target.Target]; ok {
					if pkgName, ok := langCfg.Cfg["packageName"].(string); ok {
						packageName = pkgName
					}
				}
			}

			pypiTrustedPublishingConfigs[name] = PyPITrustedPublishingConfig{
				target:          target,
				workflowDir:     filepath.Join(rootDir, actionWorkingDir),
				actionPath:      actionPath,
				publishFileName: filepath.Base(publishPath),
				packageName:     packageName,
				remoteURL:       remoteURL,
			}
		}
	}
	agenda = append(agenda, getPyPITrustedPublishingInstructions(ctx, pypiTrustedPublishingConfigs)...)

	logger.Println(styles.Info.Render("Files successfully generated!\n"))
	for _, statusMsg := range status {
		logger.Println(styles.Info.Render(fmt.Sprintf("• %s", statusMsg)))
	}
	logger.Println(styles.Info.Render("\n"))

	msg := styles.RenderInstructionalMessage("For your publishing setup to be complete perform the following steps.",
		agenda...)
	logger.Println(msg)

	return nil
}

type NPMTrustedPublishingConfig struct {
	target          workflow.Target
	workflowDir     string
	actionPath      string
	publishFileName string
	packageName     string
	remoteURL       string
}

type PyPITrustedPublishingConfig struct {
	target          workflow.Target
	workflowDir     string
	actionPath      string
	publishFileName string
	packageName     string
	remoteURL       string
}

func getNPMTrustedPublishingInstructions(_ context.Context, npmConfigs map[string]NPMTrustedPublishingConfig) []string {
	var agenda []string

	// Collect unique action paths
	actionPaths := make(map[string][]string)
	for _, npmConfig := range npmConfigs {
		if _, exists := actionPaths[npmConfig.actionPath]; !exists {
			actionPaths[npmConfig.actionPath] = []string{}
		}
		actionPaths[npmConfig.actionPath] = append(actionPaths[npmConfig.actionPath], npmConfig.packageName)
	}

	for targetName, npmConfig := range npmConfigs {
		repoOwner := "<user>"
		repoName := "<repository>"
		if npmConfig.remoteURL != "" {
			// Expected format: "https://github.com/<user>/<repository>"
			re := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/?$`)
			matches := re.FindStringSubmatch(npmConfig.remoteURL)
			if len(matches) == 3 {
				repoOwner = matches[1]
				repoName = matches[2]
			}
		}

		if len(npmConfigs) == 1 {
			agenda = append(agenda, fmt.Sprintf("• Access your newly published package's settings at https://www.npmjs.com/package/%s/access", npmConfig.packageName))
		} else {
			agenda = append(agenda, fmt.Sprintf("• [%s] Access the '%s' package's settings at https://www.npmjs.com/package/%s/access", strings.ToUpper(npmConfig.target.Target), targetName, npmConfig.packageName))
		}

		configLines := []string{
			fmt.Sprintf("\t\t- Organization or user: %s", repoOwner),
			fmt.Sprintf("\t\t- Repository: %s", repoName),
			fmt.Sprintf("\t\t- Workflow filename: %s", npmConfig.publishFileName),
			"\t\t- Environment name: <Leave Blank>",
		}
		agenda = append(agenda, fmt.Sprintf("\t◦ Add 'GitHub Actions' as a 'Trusted Publisher' with the following configuration:\n%s", strings.Join(configLines, "\n")))
	}

	for actionPath, packageNames := range actionPaths {
		if len(packageNames) == 1 {
			agenda = append(agenda, fmt.Sprintf("• Navigate to %s to regenerate and publish a new version of the %s package.", actionPath, packageNames[0]))
			agenda = append(agenda, fmt.Sprintf("• Your package's latest version should now include a 'Provenance' at https://www.npmjs.com/package/%s#provenance", packageNames[0]))
		} else {
			agenda = append(agenda, fmt.Sprintf("• Navigate to %s to regenerate and publish new versions of your packages.", actionPath))
			agenda = append(agenda, "• Your packages' latest versions should now be labelled with a green check mark and include a 'Provenance'.")
		}
	}

	return agenda
}

func getPyPITrustedPublishingInstructions(_ context.Context, pypiConfigs map[string]PyPITrustedPublishingConfig) []string {
	var agenda []string

	if len(pypiConfigs) == 0 {
		return agenda
	}

	// Collect unique action paths
	actionPaths := make(map[string][]string)
	for _, pypiConfig := range pypiConfigs {
		if _, exists := actionPaths[pypiConfig.actionPath]; !exists {
			actionPaths[pypiConfig.actionPath] = []string{}
		}
		actionPaths[pypiConfig.actionPath] = append(actionPaths[pypiConfig.actionPath], pypiConfig.packageName)
	}

	for targetName, pypiConfig := range pypiConfigs {
		repoOwner := "<user>"
		repoName := "<repository>"
		if pypiConfig.remoteURL != "" {
			// Expected format: "https://github.com/<user>/<repository>"
			re := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/?$`)
			matches := re.FindStringSubmatch(pypiConfig.remoteURL)
			if len(matches) == 3 {
				repoOwner = matches[1]
				repoName = matches[2]
			}
		}

		if len(pypiConfigs) == 1 {
			agenda = append(agenda, fmt.Sprintf("• Configure trusted publishing for your PyPI package '%s':", pypiConfig.packageName))
		} else {
			agenda = append(agenda, fmt.Sprintf("• [%s] Configure trusted publishing for PyPI package '%s':", strings.ToUpper(pypiConfig.target.Target), targetName))
		}

		agenda = append(agenda, fmt.Sprintf("\t◦ Navigate to https://pypi.org/manage/project/%s/settings/publishing/", pypiConfig.packageName))

		configLines := []string{
			fmt.Sprintf("\t\t- Owner: %s", repoOwner),
			fmt.Sprintf("\t\t- Repository name: %s", repoName),
			fmt.Sprintf("\t\t- Workflow name: %s", pypiConfig.publishFileName),
			"\t\t- Environment name: <Leave Blank>",
		}
		agenda = append(agenda, fmt.Sprintf("\t◦ Add a new 'trusted publisher' with the following configuration:\n%s", strings.Join(configLines, "\n")))
	}

	for actionPath, packageNames := range actionPaths {
		if len(packageNames) == 1 {
			agenda = append(agenda, fmt.Sprintf("• Navigate to %s to regenerate and publish a new version of the %s package.", actionPath, packageNames[0]))
			agenda = append(agenda, fmt.Sprintf("• Your package will be published with attestations. Verify at https://pypi.org/project/%s/#files", packageNames[0]))
		} else {
			agenda = append(agenda, fmt.Sprintf("• Navigate to %s to regenerate and publish new versions of your packages.", actionPath))
			agenda = append(agenda, "• Your packages will be published with attestations.")
		}
	}

	return agenda
}

func configureTesting(ctx context.Context, flags ConfigureTestsFlags) error {
	logger := log.From(ctx)

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := testcmd.CheckTestingEnabled(ctx); err != nil {
		return err
	}

	actionWorkingDir := getActionWorkingDirectoryFromFlag(rootDir, flags.WorkflowDirectory)

	workflowFile, workflowFilePath, _ := workflow.Load(filepath.Join(rootDir, actionWorkingDir))
	if workflowFile == nil {
		return renderAndPrintWorkflowNotFound("testing", logger)
	}

	var testingOptions []huh.Option[string]
	for name, target := range workflowFile.Targets {
		if slices.Contains(prompts.SupportedTestingTargets, target.Target) {
			testingOptions = append(testingOptions, huh.NewOption(fmt.Sprintf("%s [%s]", name, strings.ToUpper(target.Target)), name))
		}
	}

	var chosenTargets []string
	switch {
	case len(testingOptions) == 0:
		logger.Println(styles.Info.Render("No existing SDK targets support sdk testing."))
		return nil
	case len(testingOptions) == 1:
		chosenTargets = []string{testingOptions[0].Value}
	default:
		chosenTargets, err = prompts.SelectTestingTargets(testingOptions, true)
		if err != nil {
			return err
		}
	}

	if len(chosenTargets) == 0 {
		logger.Println(styles.Info.Render("No targets selected. Exiting."))
		return nil
	}

	for _, name := range chosenTargets {
		target := workflowFile.Targets[name]
		testingEnabled := true
		target.Testing = &workflow.Testing{
			Enabled: &testingEnabled,
		}
		workflowFile.Targets[name] = target
		outDir := ""
		if target.Output != nil {
			outDir = *target.Output
		}
		cfg, err := config.Load(filepath.Join(rootDir, actionWorkingDir, outDir))
		if err != nil {
			return errors.Wrapf(err, "failed to load config file for target %s", name)
		}
		cfg.Config.Generation.Tests.GenerateTests = true
		cfg.Config.Generation.Tests.GenerateNewTests = true
		if err := config.SaveConfig(filepath.Dir(cfg.ConfigPath), cfg.Config); err != nil {
			return errors.Wrapf(err, "failed to save config file for target %s", name)
		}

		// We clear out the existing generated tests gen.lock entry and arazzo file, so we can rebuild from scratch.
		if flags.Rebuild != nil && cfg.LockFile != nil {
			_ = testcmd.RebuildTests(ctx, name, *flags.Rebuild, cfg)
		}
	}

	if err := workflow.Save(filepath.Join(rootDir, actionWorkingDir), workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	generationWorkflowFilePath := filepath.Join(rootDir, ".github/workflows/sdk_generation.yaml")
	if len(workflowFile.Targets) > 1 {
		sanitizedName := strings.ReplaceAll(strings.ToLower(chosenTargets[0]), "-", "_")
		generationWorkflowFilePath = filepath.Join(rootDir, fmt.Sprintf(".github/workflows/sdk_generation_%s.yaml", sanitizedName))
	}

	generationWorkflow := &config.GenerateWorkflow{}
	var testingFilePaths []string
	hasAppAccess := false
	selectedAppInstall := false
	// configure github has been completed already and we have a PR mode workflow
	if err := prompts.ReadGenerationFile(generationWorkflow, generationWorkflowFilePath); err == nil {
		if mode, ok := generationWorkflow.Jobs.Generate.With[config.Mode].(string); ok && mode == "pr" {
			var remoteURL string
			if repo := prompts.FindGithubRepository(rootDir); repo != nil {
				remoteURL = prompts.ParseGithubRemoteURL(repo)
				if urlParts := strings.Split(remoteURL, "/"); len(urlParts) > 2 {
					hasAppAccess = checkGithubAppAccess(ctx, urlParts[len(urlParts)-2], urlParts[len(urlParts)-1])
				}
			}
			if !hasAppAccess {
				// Default to installing the app
				selectedAppInstall = true
				_, err := charm.NewForm(huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[bool]().
							Title("To run automated PR checks install the Speakeasy Github app or setup your own Github Actions PAT.").
							Description(testCheckDocs).
							Options(
								huh.NewOption("Install Speakeasy App", true),
								huh.NewOption("Setup Github PAT", false),
							).
							Value(&selectedAppInstall),
					),
				)).ExecuteForm()
				if err != nil {
					return err
				}
			}

			testingFilePaths, err = prompts.WriteTestingFiles(ctx, workflowFile, rootDir, actionWorkingDir, chosenTargets, !hasAppAccess && !selectedAppInstall)
			if err != nil {
				return err
			}
		}
	}

	status := []string{"Test definitions written to:", fmt.Sprintf("\t- %s", filepath.Join(filepath.Dir(workflowFilePath), "tests.arazzo.yaml"))}
	if len(testingFilePaths) > 0 {
		status = append(status, "GitHub action (test) files written to:")
		for _, path := range testingFilePaths {
			status = append(status, fmt.Sprintf("\t- %s", path))
		}
	}

	agenda := []string{"• Execute `speakeasy test` to run your tests locally."}
	if len(testingFilePaths) > 0 {
		if !hasAppAccess && selectedAppInstall {
			agenda = append(agenda, fmt.Sprintf("• Install - %s.", appInstallURL))
		}
		if !hasAppAccess && !selectedAppInstall {
			agenda = append(agenda, fmt.Sprintf("• Follow documentation to create your Github PAT and store it under repository secrets as %s.", styles.MakeBold("PR_CREATION_PAT")))
		}
		agenda = append(agenda, "• Push your tests and file updates to github!")
		agenda = append(agenda, fmt.Sprintf("• For more information see %s", testingSetupDocs))
	}

	if actionWorkingDir != "" {
		if err = os.Chdir(filepath.Join(rootDir, actionWorkingDir)); err != nil {
			return errors.Wrapf(err, "failed to change directory for run %s", filepath.Join(rootDir, actionWorkingDir))
		}
	}
	wf, err := run.NewWorkflow(
		ctx,
		run.WithTarget("all"),
		run.WithBoostrapTests(),
		run.WithAllowPrompts(true),
	)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	if err = wf.RunWithVisualization(ctx); err != nil {
		return errors.Wrapf(err, "failed to generate tests")
	}

	success := styles.MakeBoxed(styles.MakeBold(fmt.Sprintf("✅ %s ✅", styles.Info.Render("Tests Successfully Generated"))), styles.Colors.Green, lipgloss.Center)
	logger.Println(success + "\n")

	for _, statusMsg := range status {
		logger.Println(styles.Info.Render(statusMsg))
	}
	logger.Println(styles.Info.Render("\n"))

	msg := styles.RenderInstructionalMessage("For your testing setup to complete perform the following steps.",
		agenda...)
	logger.Println(msg)

	return nil
}

func configureGithub(ctx context.Context, flags ConfigureGithubFlags) error {
	logger := log.From(ctx)
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	orgSlug := core.GetOrgSlugFromContext(ctx)
	workspaceSlug := core.GetWorkspaceSlugFromContext(ctx)
	actionWorkingDir := getActionWorkingDirectoryFromFlag(rootDir, flags.WorkflowDirectory)

	workflowFile, workflowFilePath, _ := workflow.Load(filepath.Join(rootDir, actionWorkingDir))
	if workflowFile == nil {
		return renderAndPrintWorkflowNotFound("github", logger)
	}
	ctx = events.SetTargetInContext(ctx, rootDir)

	// check if the git repository is a github URI
	event := shared.CliEvent{}
	events.EnrichEventWithGitMetadata(ctx, &event)

	// Installing the app is only relevant if we are in a remote linked github repository
	hasAppAccess := false
	if event.GitRemoteDefaultOwner != nil && *event.GitRemoteDefaultOwner != "" && event.GitRemoteDefaultRepo != nil && *event.GitRemoteDefaultRepo != "" {
		hasAppAccess = checkGithubAppAccess(ctx, *event.GitRemoteDefaultOwner, *event.GitRemoteDefaultRepo)
		if !hasAppAccess {
			continueAfterInstall := false
			_, err := charm.NewForm(huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[bool]().
						Title("\n\nSpeakeasy has a Github app that can help you set up your SDK repo from speakeasy configure.\nWould you like to install it?\n").
						Options(
							huh.NewOption("Yes", true),
							huh.NewOption("No", false),
						).
						Value(&continueAfterInstall),
				),
			)).ExecuteForm()
			if err != nil {
				return err
			}

			if continueAfterInstall {
				_ = utils.OpenInBrowser(appInstallURL)
				logger.Println(styles.Info.Render("Install the Github App then continue with `speakeasy configure github`!\n"))
				return nil
			}
		}
	}

	secrets := make(map[string]string)
	var generationWorkflowFilePaths []string
	var isPRMode bool

	if len(workflowFile.Targets) <= 1 {
		generationWorkflow, generationWorkflowFilePath, err := writeGenerationFile(workflowFile, rootDir, actionWorkingDir, nil)
		if err != nil {
			return err
		}

		for key, val := range generationWorkflow.Jobs.Generate.Secrets {
			secrets[key] = val
		}

		if mode, ok := generationWorkflow.Jobs.Generate.With[config.Mode].(string); ok && mode == "pr" {
			isPRMode = true
		}

		generationWorkflowFilePaths = append(generationWorkflowFilePaths, generationWorkflowFilePath)
	} else {
		for name := range workflowFile.Targets {
			generationWorkflow, generationWorkflowFilePath, err := writeGenerationFile(workflowFile, rootDir, actionWorkingDir, &name)
			if err != nil {
				return err
			}

			for key, val := range generationWorkflow.Jobs.Generate.Secrets {
				secrets[key] = val
			}

			if mode, ok := generationWorkflow.Jobs.Generate.With[config.Mode].(string); ok && mode == "pr" {
				isPRMode = true
			}

			generationWorkflowFilePaths = append(generationWorkflowFilePaths, generationWorkflowFilePath)
		}
	}

	if err := workflow.Save(filepath.Join(rootDir, actionWorkingDir), workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	autoConfigureRepoSuccess := false
	if hasAppAccess {
		autoConfigureRepoSuccess = configureGithubRepo(ctx, *event.GitRemoteDefaultOwner, *event.GitRemoteDefaultRepo)
	}

	var remoteURL string
	if repo := prompts.FindGithubRepository(rootDir); repo != nil {
		remoteURL = prompts.ParseGithubRemoteURL(repo)
	}

	secretPath := repositorySecretPath
	if remoteURL != "" {
		secretPath = fmt.Sprintf("%s/settings/secrets/actions", remoteURL)
	}

	status := []string{
		fmt.Sprintf("Speakeasy workflow written to - %s", workflowFilePath),
	}
	status = append(status, "GitHub action (generate+publish) written to:")
	for _, path := range generationWorkflowFilePaths {
		status = append(status, fmt.Sprintf("\t- %s", path))
	}

	agenda := []string{}
	// This attribute is nil when not in a git repository
	if event.GitRelativeCwd == nil {
		agenda = append(agenda, "• Initialize your Git Repository - https://github.com/git-guides/git-init")
	}
	// this attribute is nil when the remote isn't github
	if event.GitRemoteDefaultOwner == nil {
		agenda = append(agenda, "• Configure your GitHub remote - https://docs.github.com/en/get-started/getting-started-with-git/managing-remote-repositories")
	}

	actionPath := actionsPath
	if remoteURL != "" {
		actionPath = fmt.Sprintf("%s/actions", remoteURL)
	}

	actionSettingsPath := actionsSettingsPath
	if remoteURL != "" {
		actionSettingsPath = fmt.Sprintf("%s/settings/actions", remoteURL)
	}

	if !autoConfigureRepoSuccess {
		agenda = append(agenda, fmt.Sprintf("• Setup a Speakeasy API Key as a GitHub Secret - %s/org/%s/%s/settings/api-keys", core.GetServerURL(), orgSlug, workspaceSlug))
	}

	if len(secrets) > 2 || !autoConfigureRepoSuccess {
		agenda = append(agenda, fmt.Sprintf("• On GitHub navigate to %s and set up the following repository secrets:", secretPath))
	}

	for key := range secrets {
		if key != config.GithubAccessToken && (key != config.SpeakeasyApiKey || !autoConfigureRepoSuccess) {
			agenda = append(agenda, fmt.Sprintf("\t◦ Provide a secret with name %s", styles.MakeBold(strings.ToUpper(key))))
		}
	}
	if isPRMode {
		agenda = append(agenda, fmt.Sprintf("• Navigate to %s ensure `Workflow permissions: can create pull requests` is enabled.", actionSettingsPath))
	}
	agenda = append(agenda, fmt.Sprintf("• Push your repository to github! Navigate to %s to view your generations.", actionPath))

	logger.Println(styles.Info.Render("Files successfully generated!\n"))
	for _, statusMsg := range status {
		logger.Println(styles.Info.Render(fmt.Sprintf("• %s", statusMsg)))
	}
	logger.Println(styles.Info.Render("\n"))

	msg := styles.RenderInstructionalMessage("For your github workflow setup to be complete perform the following steps.",
		agenda...)
	logger.Println(msg)

	logger.Println(styles.Info.Render("\n\n" + fmt.Sprintf("For more information see - %s", githubSetupDocs)))

	return nil
}

func writeGenerationFile(workflowFile *workflow.Workflow, workingDir, workflowFileDir string, target *string) (*config.GenerateWorkflow, string, error) {
	generationWorkflowFilePath := filepath.Join(workingDir, ".github/workflows/sdk_generation.yaml")
	if target != nil {
		sanitizedName := strings.ReplaceAll(strings.ToLower(*target), "-", "_")
		generationWorkflowFilePath = filepath.Join(workingDir, fmt.Sprintf(".github/workflows/sdk_generation_%s.yaml", sanitizedName))
	}

	if _, err := os.Stat(filepath.Join(workingDir, ".github/workflows")); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(workingDir, ".github/workflows"), 0o755)
		if err != nil {
			return nil, "", err
		}
	}

	generationWorkflow := &config.GenerateWorkflow{}
	_ = prompts.ReadGenerationFile(generationWorkflow, generationWorkflowFilePath)

	generationWorkflow, err := prompts.ConfigureGithub(generationWorkflow, workflowFile, workflowFileDir, target)
	if err != nil {
		return nil, "", err
	}

	if err = prompts.WriteGenerationFile(generationWorkflow, generationWorkflowFilePath); err != nil {
		return nil, "", errors.Wrapf(err, "failed to write github workflow file")
	}

	return generationWorkflow, generationWorkflowFilePath, nil
}

type targetWorkflowPaths struct {
	generationWorkflowPath string
	publishWorkflowPaths   []string
}

func writePublishingFile(wf *workflow.Workflow, target workflow.Target, targetName, currentWorkingDir, workflowFileDir string) (*config.GenerateWorkflow, targetWorkflowPaths, error) {
	paths := targetWorkflowPaths{}
	paths.generationWorkflowPath = filepath.Join(currentWorkingDir, ".github/workflows/sdk_generation.yaml")
	if len(wf.Targets) > 1 {
		sanitizedName := strings.ReplaceAll(strings.ToLower(targetName), "-", "_")
		paths.generationWorkflowPath = filepath.Join(currentWorkingDir, fmt.Sprintf(".github/workflows/sdk_generation_%s.yaml", sanitizedName))
	}

	if _, err := os.Stat(filepath.Join(currentWorkingDir, ".github/workflows")); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(currentWorkingDir, ".github/workflows"), 0o755)
		if err != nil {
			return nil, paths, err
		}
	}

	generationWorkflow := &config.GenerateWorkflow{}
	if err := prompts.ReadGenerationFile(generationWorkflow, paths.generationWorkflowPath); err != nil {
		return nil, paths, fmt.Errorf("you cannot run configure publishing when a github workflow file %s does not exist, try speakeasy configure github", paths.generationWorkflowPath)
	}

	publishPaths, err := prompts.WritePublishing(wf, generationWorkflow, targetName, currentWorkingDir, workflowFileDir, target)
	if err != nil {
		return nil, paths, errors.Wrapf(err, "failed to write publishing configs")
	}

	paths.publishWorkflowPaths = publishPaths
	if err = prompts.WriteGenerationFile(generationWorkflow, paths.generationWorkflowPath); err != nil {
		return nil, paths, errors.Wrapf(err, "failed to write github workflow file")
	}

	return generationWorkflow, paths, nil
}

func handleLegacySDKTarget(workingDir string, workflowFile *workflow.Workflow) ([]string, []huh.Option[string]) {
	if cfg, err := config.Load(workingDir); err == nil && cfg.Config != nil && len(cfg.Config.Languages) > 0 {
		var targetLanguage string
		for lang := range cfg.Config.Languages {
			// A problem with some old gen.yaml files pulling in non language entries
			if slices.Contains(generate.GetSupportedTargetNames(), lang) {
				targetLanguage = lang
				if lang == "docs" {
					break
				}
			}
		}

		if targetLanguage != "" {
			var firstSourceName string
			for name := range workflowFile.Sources {
				firstSourceName = name
				break
			}

			workflowFile.Targets[targetLanguage] = workflow.Target{
				Target: targetLanguage,
				Source: firstSourceName,
			}
			return []string{targetLanguage}, []huh.Option[string]{huh.NewOption(charm.FormatEditOption(targetLanguage), targetLanguage)}
		}
	}

	return []string{}, []huh.Option[string]{huh.NewOption(charm.FormatNewOption("New Target"), "new target")}
}

func checkGithubAppAccess(ctx context.Context, org, repo string) bool {
	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return false
	}

	res, err := s.Github.CheckAccess(ctx, operations.CheckGithubAccessRequest{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		return false
	}

	return res.StatusCode == http.StatusOK
}

func configureGithubRepo(ctx context.Context, org, repo string) bool {
	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return false
	}

	res, err := s.Github.ConfigureTarget(ctx, shared.GithubConfigureTargetRequest{
		Org:      org,
		RepoName: repo,
	})
	if err != nil {
		return false
	}

	return res.StatusCode == http.StatusOK
}

func getActionWorkingDirectoryFromFlag(rootDir string, workflowDir string) string {
	var actionWorkingDir string
	if workflowDir != "" {
		if workflowFileDir, err := filepath.Abs(workflowDir); err == nil {
			if filepath.Base(workflowFileDir) == "workflow.yaml" {
				workflowFileDir = filepath.Dir(workflowFileDir)
			}

			if filepath.Base(workflowFileDir) == ".speakeasy" {
				workflowFileDir = filepath.Dir(workflowFileDir)
			}

			actionWorkingDir, _ = filepath.Rel(rootDir, workflowFileDir)
			// filepath.Rel returns . on an equivalent path
			if actionWorkingDir == "." || actionWorkingDir == "./" {
				actionWorkingDir = ""
			}
		}
	}

	return actionWorkingDir
}

func configureLocalWorkflow(ctx context.Context, flags ConfigureLocalWorkflowFlags) error {
	logger := log.From(ctx)

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	actionWorkingDir := getActionWorkingDirectoryFromFlag(workingDir, flags.WorkflowDirectory)
	workflowDir := filepath.Join(workingDir, actionWorkingDir)

	localWorkflowPath := filepath.Join(workflowDir, ".speakeasy", "workflow.local.yaml")

	// Check if workflow.yaml exists first
	workflowPath := filepath.Join(workflowDir, ".speakeasy", "workflow.yaml")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return renderAndPrintWorkflowNotFound("local-workflow", logger)
	}

	// Check if workflow.local.yaml already exists
	if _, err := os.Stat(localWorkflowPath); err == nil {
		logger.Println(styles.Info.Render(fmt.Sprintf("workflow.local.yaml already exists at %s", localWorkflowPath)))
		logger.Println(styles.Info.Render("Remove the existing file if you want to regenerate it."))
		return nil
	}

	if err := run.CreateWorkflowLocalFile(workflowDir); err != nil {
		return errors.Wrapf(err, "failed to create workflow.local.yaml")
	}

	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.Colors.Green).Padding(0, 1)
	successMsg := fmt.Sprintf("Successfully created workflow.local.yaml 🎉\n\nLocation: %s\n\nYou can now uncomment and modify any field in this file to override values from workflow.yaml for local development.\n\nNote: This file is for local use only — make sure to add it to .gitignore.\nFor shared workflows (e.g. CI), define targets directly in workflow.yaml.", localWorkflowPath)
	success := styles.Success.Render(successMsg)
	logger.PrintfStyled(boxStyle, "%s", success)

	return nil
}

func renderAndPrintWorkflowNotFound(cmd string, logger log.Logger) error {
	msg := styles.RenderErrorMessage("we couldn't find your Speakeasy workflow file (*.speakeasy/workflow.yaml*)",
		lipgloss.Left,
		[]string{
			"Please do one of the following:",
			"• Navigate to the root of your SDK repo",
			"• If *.speakeasy/workflow.yaml* is not in the root of your SDK repo:",
			fmt.Sprintf("\t◦ run *speakeasy configure %s -d /path/to/workflow*", cmd),
		}...)
	logger.Println(msg)
	return ErrWorkflowFileNotFound
}
