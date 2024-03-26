package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"

	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/errors"
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
	appInstallationLink  = "https://github.com/apps/speakeasy-github/installations/new"
	repositorySecretPath = "Settings > Secrets & Variables > Actions"
	actionsPath          = "Actions > Generate"
	githubSetupDocs      = "https://www.speakeasyapi.dev/docs/advanced-setup/github-setup"
)

var configureCmd = &model.CommandGroup{
	Usage:          "configure",
	Short:          "Configure your Speakeasy SDK Setup.",
	Long:           `Configure your Speakeasy SDK Setup.`,
	InteractiveMsg: "What do you want to configure?",
	Commands:       []model.Command{configureSourcesCmd, configureTargetCmd, configureGithubCmd, configurePublishingCmd},
}

type ConfigureSourcesFlags struct {
	ID  string `json:"id"`
	New bool   `json:"new"`
}

var configureSourcesCmd = &model.ExecutableCommand[ConfigureSourcesFlags]{
	Usage:        "sources",
	Short:        "Configure new or existing sources.",
	Long:         "Guided prompts to configure a new or existing source in your speakeasy workflow.",
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
	},
}

type ConfigureTargetFlags struct {
	ID  string `json:"id"`
	New bool   `json:"new"`
}

var configureTargetCmd = &model.ExecutableCommand[ConfigureTargetFlags]{
	Usage:        "targets",
	Short:        "Configure new target.",
	Long:         "Guided prompts to configure a new target in your speakeasy workflow.",
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
	},
}

type ConfigureGithubFlags struct{}

var configureGithubCmd = &model.ExecutableCommand[ConfigureGithubFlags]{
	Usage:        "github",
	Short:        "Configure Speakeasy for github.",
	Long:         "Configure your Speakeasy workflow to generate and publish from your github repo.",
	Run:          configureGithub,
	RequiresAuth: true,
}

var configurePublishingCmd = &model.ExecutableCommand[ConfigureGithubFlags]{
	Usage:        "publishing",
	Short:        "Configure Speakeasy for publishing.",
	Long:         "Configure your Speakeasy workflow to publish to package managers from your github repo.",
	Run:          configurePublishing,
	RequiresAuth: true,
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
		if _, err := charm.NewForm(huh.NewForm(prompt),
			"Let's configure a source for your workflow.").ExecuteForm(); err != nil {
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

	if err := workflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
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

	if newTarget && existingTarget == "" {
		prompt := charm.NewSelectPrompt("What target would you like to configure?", "You may choose an existing target or create a new target.", targetOptions, &existingTarget)
		if _, err := charm.NewForm(huh.NewForm(prompt),
			"Let's configure a target for your workflow.").
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
		// If a second target is added to an existing workflow file you must change the outdir of either target cannot be the root dir.
		if err := prompts.PromptForOutDirMigration(workflowFile, existingTargets); err != nil {
			return err
		}

		targetName, target, err = prompts.PromptForNewTarget(workflowFile, "", "", "")
		if err != nil {
			return err
		}

		workflowFile.Targets[targetName] = *target

		targetConfig, err = prompts.PromptForTargetConfig(targetName, target, nil, false)
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

		targetConfig, err = prompts.PromptForTargetConfig(targetName, target, existingConfig, false)
		if err != nil {
			return err
		}
	}

	outDir := workingDir
	if target.Output != nil {
		outDir = *target.Output
	}

	if err := config.SaveConfig(outDir, targetConfig); err != nil {
		return errors.Wrapf(err, "failed to save config file for target %s", targetName)
	}

	if err := workflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
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
	if workflowFile.Targets != nil && len(workflowFile.Targets) > 0 {
		successMsg += "\n\nExecute speakeasy run to regenerate your SDK!"
	}

	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.Colors.Green).Padding(0, 1)
	success := styles.Success.Render(successMsg)
	logger := log.From(ctx)
	logger.PrintfStyled(boxStyle, "%s", success)

	return nil
}

func configurePublishing(ctx context.Context, _flags ConfigureGithubFlags) error {
	logger := log.From(ctx)

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	generationWorkflowFilePath := filepath.Join(workingDir, ".github/workflows/sdk_generation.yaml")

	workflowFile, _, _ := workflow.Load(workingDir)
	if workflowFile == nil {
		return fmt.Errorf("you cannot run configure when a speakeasy workflow does not exist, try speakeasy quickstart")
	}

	generationWorkflow := &config.GenerateWorkflow{}
	if err := prompts.ReadGenerationFile(generationWorkflow, generationWorkflowFilePath); err != nil {
		return fmt.Errorf("you cannot run configure publishing when a github workflow file %s does not exist, try speakeasy configure github", generationWorkflowFilePath)
	}

	var publishingOptions []huh.Option[string]
	for name, target := range workflowFile.Targets {
		if slices.Contains(prompts.SupportedPublishingTargets, target.Target) {
			publishingOptions = append(publishingOptions, huh.NewOption(fmt.Sprintf("%s [%s]", name, strings.ToUpper(target.Target)), name))
		}
	}

	if len(publishingOptions) == 0 {
		logger.Println(styles.Info.Render("No existing SDK targets require package manager publishing configuration."))
	}

	chosenTargets, err := prompts.SelectPublishingTargets(publishingOptions, true)
	if err != nil {
		return err
	}

	for _, name := range chosenTargets {
		target := workflowFile.Targets[name]
		modifiedTarget, err := prompts.ConfigurePublishing(&target, name)
		if err != nil {
			return err
		}
		workflowFile.Targets[name] = *modifiedTarget
	}

	var publishPath string
	if len(chosenTargets) > 0 {
		generationWorkflow, publishPath, err = prompts.WritePublishing(generationWorkflow, workflowFile, workingDir)
		if err != nil {
			return errors.Wrapf(err, "failed to write publishing configs")
		}
	}

	if err = prompts.WriteGenerationFile(generationWorkflow, generationWorkflowFilePath); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
	}

	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	var remoteURL string
	if repo := prompts.FindGithubRepository(workingDir); repo != nil {
		remoteURL = prompts.ParseGithubRemoteURL(repo)
	}

	secretPath := repositorySecretPath
	if remoteURL != "" {
		secretPath = fmt.Sprintf("%s/settings/secrets/actions", remoteURL)
	}

	_, workflowFilePath, err := workflow.Load(workingDir)
	if err != nil {
		return errors.Wrapf(err, "failed to load workflow file")
	}

	status := []string{
		fmt.Sprintf("Speakeasy workflow written to - %s", workflowFilePath),
	}
	if publishPath != "" {
		status = append(status, fmt.Sprintf("GitHub action (generate) written to - %s", generationWorkflowFilePath))
		status = append(status, fmt.Sprintf("GitHub action (publish) written to - %s", publishPath))
	} else {
		status = append(status, fmt.Sprintf("GitHub action (generate+publish) written to - %s", generationWorkflowFilePath))
	}

	var agenda []string
	for key := range generationWorkflow.Jobs.Generate.Secrets {
		if key != config.GithubAccessToken {
			agenda = append(agenda, fmt.Sprintf("\t◦ Provide a secret with name %s", styles.MakeBold(strings.ToUpper(key))))
		}
	}

	logger.Println(styles.Info.Render("Files successfully generated!\n"))
	for _, statusMsg := range status {
		logger.Println(styles.Info.Render(fmt.Sprintf("• %s", statusMsg)))
	}
	logger.Println(styles.Info.Render("\n"))

	if len(agenda) != 0 {
		agenda = append([]string{
			fmt.Sprintf("• In your repo navigate to %s and setup the following repository secrets:", secretPath),
		}, agenda...)

		msg := styles.RenderInstructionalMessage("For your publishing setup to be complete perform the following steps.",
			agenda...)
		logger.Println(msg)
	}

	return nil
}

func configureGithub(ctx context.Context, _flags ConfigureGithubFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	logger := log.From(ctx)

	workspaceID, err := core.GetWorkspaceIDFromContext(ctx)
	if err != nil {
		return err
	}

	generationWorkflowFilePath := filepath.Join(workingDir, ".github/workflows/sdk_generation.yaml")

	workflowFile, _, _ := workflow.Load(workingDir)
	if workflowFile == nil {
		return fmt.Errorf("you cannot run configure when a speakeasy workflow does not exist, try speakeasy quickstart")
	}

	ctx = events.SetTargetInContext(ctx, workingDir)

	generationWorkflow := &config.GenerateWorkflow{}
	prompts.ReadGenerationFile(generationWorkflow, generationWorkflowFilePath)

	generationWorkflow, err = prompts.ConfigureGithub(generationWorkflow, workflowFile)
	if err != nil {
		return err
	}

	var publishingOptions []huh.Option[string]
	for name, target := range workflowFile.Targets {
		if slices.Contains(prompts.SupportedPublishingTargets, target.Target) {
			publishingOptions = append(publishingOptions, huh.NewOption(fmt.Sprintf("%s [%s]", name, strings.ToUpper(target.Target)), name))
		}
	}

	var chosenTargets []string
	if len(publishingOptions) > 0 {
		chosenTargets, err = prompts.SelectPublishingTargets(publishingOptions, false)
		if err != nil {
			return err
		}
	}

	for _, name := range chosenTargets {
		target := workflowFile.Targets[name]
		modifiedTarget, err := prompts.ConfigurePublishing(&target, name)
		if err != nil {
			return err
		}
		workflowFile.Targets[name] = *modifiedTarget
	}

	if _, err := os.Stat(workingDir + "/" + ".github/workflows"); os.IsNotExist(err) {
		err = os.MkdirAll(workingDir+"/"+".github/workflows", 0o755)
		if err != nil {
			return err
		}
	}

	var publishPath string
	if len(chosenTargets) > 0 {
		generationWorkflow, publishPath, err = prompts.WritePublishing(generationWorkflow, workflowFile, workingDir)
		if err != nil {
			return errors.Wrapf(err, "failed to write publishing configs")
		}
	}

	if err = prompts.WriteGenerationFile(generationWorkflow, generationWorkflowFilePath); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
	}

	if err := workflow.Save(workingDir, workflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	var remoteURL string
	if repo := prompts.FindGithubRepository(workingDir); repo != nil {
		remoteURL = prompts.ParseGithubRemoteURL(repo)
	}

	secretPath := repositorySecretPath
	if remoteURL != "" {
		secretPath = fmt.Sprintf("%s/settings/secrets/actions", remoteURL)
	}

	_, workflowFilePath, err := workflow.Load(workingDir)
	if err != nil {
		return errors.Wrapf(err, "failed to load workflow file")
	}

	status := []string{
		fmt.Sprintf("Speakeasy workflow written to - %s", workflowFilePath),
	}
	if publishPath != "" {
		status = append(status, fmt.Sprintf("GitHub action (generate) written to - %s", generationWorkflowFilePath))
		status = append(status, fmt.Sprintf("GitHub action (publish) written to - %s", publishPath))
	} else {
		status = append(status, fmt.Sprintf("GitHub action (generate+publish) written to - %s", generationWorkflowFilePath))
	}

	agenda := []string{}

	// check if the git repository is a github URI
	event := shared.CliEvent{}
	events.EnrichEventWithGitMetadata(ctx, &event)
	// This attribute is nil when not in a git repository
	if event.GitRelativeCwd == nil {
		agenda = append(agenda, fmt.Sprintf("• Initialize your Git Repository - https://github.com/git-guides/git-init"))
	}
	// this attribute is nil when the remote isn't github
	if event.GitRemoteDefaultOwner == nil {
		agenda = append(agenda, fmt.Sprintf("• Configure your GitHub remote - https://docs.github.com/en/get-started/getting-started-with-git/managing-remote-repositories"))
	}

	actionPath := actionsPath
	if remoteURL != "" {
		actionPath = fmt.Sprintf("%s/actions", remoteURL)
	}

	agenda = append(agenda, fmt.Sprintf("• Setup a Speakeasy API Key as a GitHub Secret - %s/workspaces/%s/apikeys", core.GetServerURL(), workspaceID))
	agenda = append(agenda, fmt.Sprintf("• In your repo navigate to %s and setup the following repository secrets:", secretPath))

	for key := range generationWorkflow.Jobs.Generate.Secrets {
		if key != config.GithubAccessToken {
			agenda = append(agenda, fmt.Sprintf("\t◦ Provide a secret with name %s", styles.MakeBold(strings.ToUpper(key))))
		}
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

func handleLegacySDKTarget(workingDir string, workflowFile *workflow.Workflow) ([]string, []huh.Option[string]) {
	if cfg, err := config.Load(workingDir); err == nil && cfg.Config != nil && len(cfg.Config.Languages) > 0 {
		var targetLanguage string
		for lang := range cfg.Config.Languages {
			// A problem with some old gen.yaml files pulling in non language entries
			if slices.Contains(generate.GetSupportedLanguages(), lang) {
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
