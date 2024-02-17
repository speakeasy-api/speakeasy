package prompts

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

func getBaseTargetPrompts(currentWorkflow *workflow.Workflow, sourceName, targetName, targetType, outDir *string, newTarget bool) *huh.Group {
	targetFields := []huh.Field{}
	if newTarget {
		targetFields = append(targetFields, huh.NewSelect[string]().
			Title("Which target would you like to generate?").
			Description("Choose from this list of supported generation targets. \n").
			Options(huh.NewOptions(GetSupportedTargets()...)...).
			Value(targetType))
	}

	if !newTarget || targetName == nil || *targetName == "" {
		originalTargetName := ""
		if targetName != nil {
			originalTargetName = *targetName
		}

		targetFields = append(targetFields,
			charm.NewInput().
				Title("What is a good name for this target?").
				Validate(func(s string) error {
					if _, ok := currentWorkflow.Targets[s]; ok && s != originalTargetName {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(targetName),
		)
	}

	targetFields = append(targetFields, rendersSelectSource(currentWorkflow, sourceName)...)
	if len(currentWorkflow.Targets) > 0 {
		targetFields = append(targetFields,
			charm.NewInput().
				Title("What is a good output directory for your generation target?").
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
				}).
				Value(outDir),
		)
	}

	return huh.NewGroup(targetFields...)
}

func targetBaseForm(quickstart *Quickstart) (*QuickstartState, error) {
	var targetName string
	if len(quickstart.WorkflowFile.Targets) == 0 {
		targetName = "first-target"
	}

	var targetType string
	if quickstart.Defaults.TargetType != nil {
		targetType = *quickstart.Defaults.TargetType
	}

	targetName, target, err := PromptForNewTarget(quickstart.WorkflowFile, targetName, targetType, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new target")
	}

	if err := target.Validate(generate.GetSupportedLanguages(), quickstart.WorkflowFile.Sources); err != nil {
		return nil, errors.Wrap(err, "failed to validate target")
	}

	quickstart.WorkflowFile.Targets[targetName] = *target

	nextState := ConfigBase

	return &nextState, nil
}

func PromptForNewTarget(currentWorkflow *workflow.Workflow, targetName, targetType, outDir string) (string, *workflow.Target, error) {
	sourceName := getSourcesFromWorkflow(currentWorkflow)[0]
	prompts := getBaseTargetPrompts(currentWorkflow, &sourceName, &targetName, &targetType, &outDir, true)
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(prompts),
		"Let's setup a new target for your workflow.",
		"A target is a set of workflow instructions and a gen.yaml config that defines what you would like to generate.")).
		Run(); err != nil {
		return "", nil, err
	}

	target := workflow.Target{
		Target: targetType,
		Source: sourceName,
	}
	if outDir != "" {
		target.Output = &outDir
	}

	if err := target.Validate(generate.GetSupportedLanguages(), currentWorkflow.Sources); err != nil {
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
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(prompts),
		"Let's setup a new target for your workflow.",
		"A target is a set of workflow instructions and a gen.yaml config that defines what you would like to generate.")).
		Run(); err != nil {
		return "", nil, err
	}

	newTarget := workflow.Target{
		Target: targetType,
		Source: sourceName,
	}
	if outDir != "" {
		newTarget.Output = &outDir
	}

	if err := newTarget.Validate(generate.GetSupportedLanguages(), currentWorkflow.Sources); err != nil {
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
			outDir := ""
			if target.Output != nil {
				outDir = *target.Output
			}
			originalDir := outDir

			if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
				huh.NewGroup(charm.NewInput().
					Title(fmt.Sprintf("Provide an output directory for your generation target %s.", targetName)).
					Validate(func(s string) error {
						if currentDir(s) {
							return fmt.Errorf("the output dir must not be in the root folder")
						}

						return nil
					}).
					Value(&outDir))),
				"To setup multiple targets you must select an output directory not in the root folder.")).
				Run(); err != nil {
				return err
			}

			target.Output = &outDir
			currentWorkflow.Targets[targetName] = target

			if err := moveOutDir(outDir, originalDir); err != nil {
				return errors.Wrap(err, "failed to move out dir")
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

	newSpeakeasyFolderPath := workingDir + "/" + outDir + "/" + ".speakeasy"
	existingSpeakeasyFolderPath := workingDir + "/" + previousDir + "/" + ".speakeasy"
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

	if _, err := os.Stat(existingSpeakeasyFolderPath + "/" + "gen.lock"); err == nil {
		if err := utils.MoveFile(existingSpeakeasyFolderPath+"/"+"gen.lock", newSpeakeasyFolderPath+"/"+"gen.lock"); err != nil {
			return errors.Wrapf(err, "failed to copy config file")
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
