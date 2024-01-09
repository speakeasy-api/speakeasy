package quickstart

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/charm"
)

func targetBaseForm(inputWorkflow *workflow.Workflow) (*State, error) {
	var targetName, targetType, sourceName, outputLocation string
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("A name for this target:").
				Placeholder("unique name across this workflow").
				Validate(func(s string) error {
					if _, ok := inputWorkflow.Targets[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Inline(true).
				Prompt(" ").
				Value(&targetName),
			huh.NewSelect[string]().
				Title("What target would you like to generate:").
				Description("Choose from this list of supported generation targets. \n").
				Options(huh.NewOptions(getSupportedTargets()...)...).
				Value(&targetType),
			huh.NewSelect[string]().
				Title("What source would you like to generate this target from:").
				Description("Choose from this list of existing sources \n").
				Options(huh.NewOptions(getSourcesFromWorkflow(inputWorkflow)...)...).
				Value(&sourceName),
			huh.NewInput().
				Title("Provide an output location for your generation target (OPTIONAL):").
				Inline(true).
				Prompt(" ").
				Value(&outputLocation),
		)),
		"Let's setup a new target for your workflow.",
		"A target is a set of workflow instructions and a gen.yaml config that defines what you would like to generate.")).
		Run(); err != nil {
		return nil, err
	}

	target := workflow.Target{
		Target: targetType,
		Source: sourceName,
	}

	if outputLocation != "" {
		target.Output = &outputLocation
	}

	if err := target.Validate(generate.GetSupportedLanguages(), inputWorkflow.Sources); err != nil {
		return nil, errors.Wrap(err, "failed to validate source")
	}

	inputWorkflow.Targets[targetName] = target
	// TODO: Should we generate initial gen.yaml files for targets as well.

	addAnotherTarget, err := charm.NewBranchCondition("Would you like to add another target to your workflow file?")
	if err != nil {
		return nil, err
	}

	var nextState State = Complete
	if addAnotherTarget {
		nextState = TargetBase
	}

	return &nextState, nil
}
