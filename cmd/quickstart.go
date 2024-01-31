package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Guided setup to help you create a new SDK in minutes.",
	Long:  `Guided setup to help you create a new SDK in minutes.`,
	RunE:  quickstartExec,
}

func quickstartInit() {
	quickstartCmd.Flags().BoolP("compile", "c", true, "run SDK validation and generation after quickstart")
	quickstartCmd.Flags().StringP("schema", "s", "", "local filepath or URL for the OpenAPI schema")
	quickstartCmd.Flags().StringP("out-dir", "o", "", "output directory for the quickstart command")
	quickstartCmd.Flags().StringP("target", "t", "", fmt.Sprintf("language to generate sdk for (available options: [%s])", strings.Join(prompts.GetSupportedTargets(), ", ")))
	rootCmd.AddCommand(quickstartCmd)
}

func quickstartExec(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	shouldCompile, err := cmd.Flags().GetBool("compile")
	if err != nil {
		return err
	}

	schemaPath, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	targetType, err := cmd.Flags().GetString("target")
	if err != nil {
		return err
	}

	outDir, err := cmd.Flags().GetString("out-dir")
	if err != nil {
		return err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if workflowFile, _, _ := workflow.Load(workingDir); workflowFile != nil {
		return fmt.Errorf("you cannot run quickstart when a speakeasy workflow already exists, try speakeasy configure instead")
	}

	fmt.Println(charm.FormatCommandTitle("Welcome to the Speakeasy!",
		"Speakeasy Quickstart guides you to build a generation workflow for any combination of sources and targets. \n"+
			"After completing these steps you will be ready to start customizing and generating your SDKs.") + "\n\n\n")

	quickstartObj := prompts.Quickstart{
		WorkflowFile: &workflow.Workflow{
			Version: workflow.WorkflowVersion,
			Sources: make(map[string]workflow.Source),
			Targets: make(map[string]workflow.Target),
		},
		LanguageConfigs: make(map[string]*config.Configuration),
	}

	if schemaPath != "" {
		quickstartObj.Defaults.SchemaPath = &schemaPath
	}

	if targetType != "" {
		quickstartObj.Defaults.TargetType = &targetType
	}

	nextState := prompts.SourceBase
	for nextState != prompts.Complete {
		stateFunc := prompts.StateMapping[nextState]
		state, err := stateFunc(&quickstartObj)
		if err != nil {
			return err
		}
		nextState = *state
	}

	if err := quickstartObj.WorkflowFile.Validate(generate.GetSupportedLanguages()); err != nil {
		return errors.Wrapf(err, "failed to validate workflow file")
	}

	for _, target := range quickstartObj.WorkflowFile.Targets {
		if outDir == "" {
			outDir = workingDir + "/" + defaultOutDir(target.Target)
			break
		}
	}

	var resolvedSchema string
	for _, source := range quickstartObj.WorkflowFile.Sources {
		resolvedSchema = source.Inputs[0].Location
	}

	speakeasyFolderPath := outDir + "/" + ".speakeasy"
	if _, err := os.Stat(speakeasyFolderPath); os.IsNotExist(err) {
		err = os.MkdirAll(speakeasyFolderPath, 0o755)
		if err != nil {
			return err
		}
	}

	if err := workflow.Save(outDir, quickstartObj.WorkflowFile); err != nil {
		return errors.Wrapf(err, "failed to save workflow file")
	}

	for key, outConfig := range quickstartObj.LanguageConfigs {
		if err := config.SaveConfig(outDir, outConfig); err != nil {
			return errors.Wrapf(err, "failed to save config file for target %s", key)
		}
	}

	// If we are referencing a local schema, copy it to the output directory
	if _, err := os.Stat(resolvedSchema); err == nil {
		if err := utils.CopyFile(resolvedSchema, outDir+"/"+resolvedSchema); err != nil {
			return errors.Wrapf(err, "failed to copy schema file")
		}
	}

	// Write a github workflow file.
	var genWorkflowBuf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&genWorkflowBuf)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(quickstartObj.GithubWorkflow); err != nil {
		return errors.Wrapf(err, "failed to encode workflow file")
	}

	if _, err := os.Stat(outDir + "/" + ".github/workflows"); os.IsNotExist(err) {
		err = os.MkdirAll(outDir+"/"+".github/workflows", 0o755)
		if err != nil {
			return err
		}
	}

	if err = os.WriteFile(outDir+"/"+".github/workflows/speakeasy_sdk_generation.yaml", genWorkflowBuf.Bytes(), 0o644); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
	}

	var initialTarget string
	for key := range quickstartObj.WorkflowFile.Targets {
		initialTarget = key
		break
	}

	if shouldCompile {
		// Change working directory to our output directory
		if err := os.Chdir(outDir); err != nil {
			return errors.Wrapf(err, "failed to run speakeasy generate")
		}

		if err = run.RunWithVisualization(cmd.Context(), initialTarget, "", genVersion, "", "", "", false); err != nil {
			return errors.Wrapf(err, "failed to run speakeasy generate")
		}
	}

	return nil
}

func defaultOutDir(target string) string {
	switch target {
	case "terraform":
		return "terraform-provider-speakeasy"
	default:
	}

	return target
}
