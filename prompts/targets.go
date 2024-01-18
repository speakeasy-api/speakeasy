package prompts

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

func getBaseTargetPrompts(currentWorkflow *workflow.Workflow, sourceName, targetName, targetType, outputLocation *string) *huh.Group {
	targetFields := []huh.Field{
		huh.NewSelect[string]().
			Title("What kind target would you like to generate?").
			Description("Choose from this list of supported generation targets. \n").
			Options(huh.NewOptions(getSupportedTargets()...)...).
			Value(targetType),
		huh.NewInput().
			Title("What is a good name for this target?").
			Validate(func(s string) error {
				if _, ok := currentWorkflow.Targets[s]; ok {
					return fmt.Errorf("a source with the name %s already exists", s)
				}
				return nil
			}).
			Inline(true).
			Prompt(" ").
			Value(targetName),
	}
	targetFields = append(targetFields, rendersSelectSource(currentWorkflow, sourceName)...)
	targetFields = append(targetFields,
		huh.NewInput().
			Title("Optionally provide an output location for your generation target:").
			Placeholder("defaults to current directory").
			Inline(true).
			Prompt(" ").
			Value(outputLocation),
	)

	return huh.NewGroup(targetFields...)
}

func targetBaseForm(quickstart *Quickstart) (*QuickstartState, error) {
	var targetName string
	if len(quickstart.WorkflowFile.Targets) == 0 {
		targetName = "first-target"
	}

	targetName, target, err := PromptForNewTarget(quickstart.WorkflowFile, targetName)
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

func PromptForNewTarget(currentWorkflow *workflow.Workflow, targetName string) (string, *workflow.Target, error) {
	var targetType, outputLocation string
	sourceName := getSourcesFromWorkflow(currentWorkflow)[0]
	prompts := getBaseTargetPrompts(currentWorkflow, &sourceName, &targetName, &targetType, &outputLocation)
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

	if outputLocation != "" {
		target.Output = &outputLocation
	}

	if err := target.Validate(generate.GetSupportedLanguages(), currentWorkflow.Sources); err != nil {
		return "", nil, errors.Wrap(err, "failed to validate source")
	}

	return targetName, &target, nil
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
