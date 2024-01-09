package quickstart

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

func targetBaseForm(inputWorkflow *workflow.Workflow) (*State, error) {
	var targetName, targetType, sourceName, outputLocation string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What is a good name for this target?").
				Validate(func(s string) error {
					if _, ok := inputWorkflow.Targets[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(&targetName),
			huh.NewSelect[string]().
				Title("What target would you like to generate?").
				Options(huh.NewOptions(getSupportedTargets()...)...).
				Value(&targetType),
			huh.NewSelect[string]().
				Title("What source would you like to use when generating this target?").
				Options(huh.NewOptions(getSourcesFromWorkflow(inputWorkflow)...)...).
				Value(&sourceName),
			huh.NewInput().
				Title("Provide an output location for your generation target (OPTIONAL).").
				Value(&outputLocation),
		)).WithTheme(theme).
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

	addAnotherTarget, err := newBranchCondition("Would you like to add another target to your workflow file?")
	if err != nil {
		return nil, err
	}

	var nextState State = Complete
	if addAnotherTarget {
		nextState = TargetBase
	}

	return &nextState, nil
}
