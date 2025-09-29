package prompts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/targets"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

const (
	TargetNameDefault = "my-first-target"

	targetGroupMCP       = "mcp"
	targetGroupSDK       = "sdk"
	targetGroupTerraform = "terraform"
)

func getBaseTargetPrompts(currentWorkflow *workflow.Workflow, sourceName, targetName, targetType, outDir *string, newTarget bool) []*huh.Group {
	groups := []*huh.Group{}
	targetFields := []huh.Field{}

	if !newTarget || targetName == nil || *targetName == "" {
		originalTargetName := ""
		if targetName != nil {
			originalTargetName = *targetName
		}

		targetFields = append(targetFields,
			charm.NewInlineInput(targetName).
				Title("What is a good name for this target?").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("a target name must be provided")
					}

					if strings.Contains(s, " ") {
						return fmt.Errorf("a target name must not contain spaces")
					}

					if _, ok := currentWorkflow.Targets[s]; ok && s != originalTargetName {
						return fmt.Errorf("a target with the name %s already exists", s)
					}
					return nil
				}),
		)
	}

	targetFields = append(targetFields, rendersSelectSource(currentWorkflow, sourceName)...)
	if len(targetFields) > 0 {
		groups = append(groups, huh.NewGroup(targetFields...))
	}

	if len(currentWorkflow.Targets) > 0 {
		groups = append(groups,
			huh.NewGroup(charm.NewInlineInput(outDir).
				Title("What is a good output directory for your generation target?").
				Suggestions(charm.DirsInCurrentDir(*outDir)).
				SetSuggestionCallback(charm.SuggestionCallback(charm.SuggestionCallbackConfig{IsDirectories: true})).
				Validate(func(s string) error {
					var enforceNewDir bool
					if newTarget {
						enforceNewDir = len(currentWorkflow.Targets) > 0
					} else {
						enforceNewDir = len(currentWorkflow.Targets) > 1
					}
					if enforceNewDir && currentDir(s) {
						return fmt.Errorf("the output dir must not be the root directory")
					}

					return nil
				}),
			))
	}

	return groups
}

func targetBaseForm(ctx context.Context, quickstart *Quickstart) (*QuickstartState, error) {
	var targetName string
	// This is a temporary value is will be overwritten by sdk class name later
	if len(quickstart.WorkflowFile.Targets) == 0 {
		targetName = TargetNameDefault
	}

	var targetType string
	if quickstart.Defaults.TargetType != nil {
		targetType = *quickstart.Defaults.TargetType
	}

	var target *workflow.Target

	// Check if we have a default target type from hidden flags or use prompts
	if quickstart.Defaults.TargetType != nil && *quickstart.Defaults.TargetType != "" {
		// Use the target type that was already set (e.g., from --target flag)
		sourceName := getSourcesFromWorkflow(quickstart.WorkflowFile)[0]
		target = &workflow.Target{
			Target: targetType,
			Source: sourceName,
		}
	} else {
		updatedTargetName, targetPtr, err := PromptForNewTarget(quickstart.WorkflowFile, targetName, targetType, "")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new target")
		}
		targetName = updatedTargetName
		target = targetPtr
	}

	if err := target.Validate(targets.GetTargets(), quickstart.WorkflowFile.Sources); err != nil {
		return nil, errors.Wrap(err, "failed to validate target")
	}

	if getTargetMaturity(target.Target) == "Alpha" {
		msg := styles.RenderInfoMessage(
			"This language is in `Alpha`!",
			"Generation is supported but may not be fully featured.",
			"Chat with us for access: `https://go.speakeasy.com/chat`")

		log.From(ctx).Println(msg)
		os.Exit(0)
	}

	quickstart.WorkflowFile.Targets[targetName] = *target

	nextState := ConfigBase

	return &nextState, nil
}

func PromptForNewTarget(currentWorkflow *workflow.Workflow, targetName, targetType, outDir string) (string, *workflow.Target, error) {
	sourceName := getSourcesFromWorkflow(currentWorkflow)[0]

	// Execute first form: Target group selection
	targetGroup := new(string)
	targetGroupForm := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().
		Title("What would you like to generate?").
		Options([]huh.Option[string]{
			huh.NewOption("Software Development Kit (SDK)", targetGroupSDK),
			huh.NewOption("Terraform Provider", targetGroupTerraform),
			huh.NewOption("Model Context Protocol (MCP) Server", targetGroupMCP),
		}...).
		Value(targetGroup)))

	if _, err := charm.NewForm(targetGroupForm,
		charm.WithTitle("Let's set up a new target for your workflow."),
		charm.WithDescription("A target defines what language to generate and how.")).
		ExecuteForm(); err != nil {
		return "", nil, err
	}

	// Execute second form: Specific target type selection
	targetSelectionForm := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().
		Title(func() string {
			switch *targetGroup {
			case targetGroupMCP:
				return "Which MCP Server would you like to generate?"
			case targetGroupSDK:
				return "Which SDK language would you like to generate?"
			case targetGroupTerraform:
				return "Which Terraform Provider would you like to generate?"
			default:
				return "Select target type"
			}
		}()).
		Options(func() []huh.Option[string] {
			switch *targetGroup {
			case targetGroupMCP:
				return getMCPTargetOptions()
			case targetGroupSDK:
				return getSDKTargetOptions()
			case targetGroupTerraform:
				return getTerraformTargetOptions()
			default:
				return []huh.Option[string]{}
			}
		}()...).
		Value(&targetType)))

	if _, err := charm.NewForm(targetSelectionForm,
		charm.WithTitle("Select your target type"),
		charm.WithDescription("Choose the specific implementation you'd like to generate.")).
		ExecuteForm(); err != nil {
		return "", nil, err
	}

	remainingPrompts := getBaseTargetPrompts(currentWorkflow, &sourceName, &targetName, &targetType, &outDir, true)

	// If there are any additional prompts needed to configure the target, show these.
	if len(remainingPrompts) > 0 {
		targetConfigurationForm := huh.NewForm(remainingPrompts...)

		if _, err := charm.NewForm(targetConfigurationForm,
			charm.WithTitle("Complete your target configuration"),
			charm.WithDescription("Provide additional details for your target.")).
			ExecuteForm(); err != nil {
			return "", nil, err
		}
	}

	target := workflow.Target{
		Target: targetType,
		Source: sourceName,
	}
	if outDir != "" {
		target.Output = &outDir
	}

	if err := target.Validate(targets.GetTargets(), currentWorkflow.Sources); err != nil {
		return "", nil, errors.Wrap(err, "failed to validate target")
	}

	return targetName, &target, nil
}

func PromptForExistingTarget(currentWorkflow *workflow.Workflow, targetName string) (string, *workflow.Target, error) {
	target, _ := currentWorkflow.Targets[targetName]
	sourceName := target.Source
	targetType := target.Target
	outDir := ""
	if target.Output != nil {
		outDir = *target.Output
	}
	originalDir := outDir

	prompts := getBaseTargetPrompts(currentWorkflow, &sourceName, &targetName, &targetType, &outDir, false)
	if _, err := charm.NewForm(huh.NewForm(prompts...),
		charm.WithTitle("Let's set up a new target for your workflow."),
		charm.WithDescription("A target is a set of workflow instructions and a gen.yaml config that defines what you would like to generate.")).ExecuteForm(); err != nil {
		return "", nil, err
	}

	newTarget := workflow.Target{
		Target: targetType,
		Source: sourceName,
	}
	if outDir != "" {
		newTarget.Output = &outDir
	}

	if err := newTarget.Validate(targets.GetTargets(), currentWorkflow.Sources); err != nil {
		return "", nil, errors.Wrap(err, "failed to validate target")
	}

	if originalDir != outDir {
		if err := moveOutDir(outDir, originalDir); err != nil {
			return "", nil, errors.Wrap(err, "failed to move out dir")
		}
	}

	return targetName, &newTarget, nil
}

func PromptForOutDirMigration(currentWorkflow *workflow.Workflow, existingTargets []string) error {
	for _, targetName := range existingTargets {
		if target, ok := currentWorkflow.Targets[targetName]; ok && (target.Output == nil || currentDir(*target.Output)) {
			targetType := target.Target
			outDir := ""
			if target.Output != nil {
				outDir = *target.Output
			}
			originalDir := outDir

			if _, err := charm.NewForm(huh.NewForm(
				huh.NewGroup(charm.NewInlineInput(&outDir).
					Title(fmt.Sprintf("Optionally provide an output directory to move your existing %s target %s to.", targetType, targetName)).
					Suggestions(charm.DirsInCurrentDir(outDir)).
					SetSuggestionCallback(charm.SuggestionCallback(charm.SuggestionCallbackConfig{IsDirectories: true})))),
				charm.WithTitle("When setting up multiple targets we recommend you select an output directory not in the root folder.")).ExecuteForm(); err != nil {
				return err
			}

			if outDir != "" {
				target.Output = &outDir
				currentWorkflow.Targets[targetName] = target

				if err := moveOutDir(outDir, originalDir); err != nil {
					return errors.Wrap(err, "failed to move out dir")
				}
			}
		}
	}

	return nil
}

func moveOutDir(outDir string, previousDir string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	newDirPath, err := filepath.Abs(filepath.Join(workingDir, outDir))
	if err != nil {
		return err
	}

	previousDirPath, err := filepath.Abs(filepath.Join(workingDir, previousDir))
	if err != nil {
		return err
	}

	newSpeakeasyFolderPath := filepath.Join(newDirPath, ".speakeasy")
	existingSpeakeasyFolderPath := filepath.Join(previousDirPath, ".speakeasy")
	if newSpeakeasyFolderPath != existingSpeakeasyFolderPath {
		if _, err := os.Stat(newSpeakeasyFolderPath); os.IsNotExist(err) {
			err = os.MkdirAll(newSpeakeasyFolderPath, 0o755)
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(existingSpeakeasyFolderPath + "/" + "gen.yaml"); err == nil {
			if err := utils.MoveFile(existingSpeakeasyFolderPath+"/"+"gen.yaml", newSpeakeasyFolderPath+"/"+"gen.yaml"); err != nil {
				return errors.Wrapf(err, "failed to copy config file")
			}
		}

		if _, err := os.Stat(previousDirPath + "/" + "gen.yaml"); err == nil {
			if err := utils.MoveFile(previousDirPath+"/"+"gen.yaml", newDirPath+"/"+"gen.yaml"); err != nil {
				return errors.Wrapf(err, "failed to copy config file")
			}
		}

		if _, err := os.Stat(existingSpeakeasyFolderPath + "/" + "gen.lock"); err == nil {
			if err := utils.MoveFile(existingSpeakeasyFolderPath+"/"+"gen.lock", newSpeakeasyFolderPath+"/"+"gen.lock"); err != nil {
				return errors.Wrapf(err, "failed to copy config file")
			}
		}
	}

	return nil
}

func currentDir(dir string) bool {
	return dir == "" || dir == "." || dir == "./"
}

func rendersSelectSource(inputWorkflow *workflow.Workflow, sourceName *string) []huh.Field {
	if len(inputWorkflow.Sources) > 1 {
		return []huh.Field{
			huh.NewSelect[string]().
				Title("What source would you like to generate this target from?").
				Description("Choose from this list of existing sources \n").
				Options(huh.NewOptions(getSourcesFromWorkflow(inputWorkflow)...)...).
				Value(sourceName),
		}
	}
	return []huh.Field{}
}
