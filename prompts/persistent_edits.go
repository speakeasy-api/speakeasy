package prompts

import (
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

// PromptRunner is an interface for running prompts, allowing integration with workflow visualizers.
type PromptRunner interface {
	RunPrompt(form *huh.Form) error
}

// CustomCodeChoice represents the user's choice when prompted about custom code detection
type CustomCodeChoice string

const (
	// CustomCodeChoiceYes enables persistent edits
	CustomCodeChoiceYes CustomCodeChoice = "yes"
	// CustomCodeChoiceNo continues without enabling persistent edits
	CustomCodeChoiceNo CustomCodeChoice = "no"
	// CustomCodeChoiceDontAskAgain disables prompting permanently
	CustomCodeChoiceDontAskAgain CustomCodeChoice = "never"
)

// PromptForCustomCode prompts the user when changes are detected in generated files.
// Returns the user's choice and any error.
func PromptForCustomCode(changeSummary string) (CustomCodeChoice, error) {
	var choice CustomCodeChoice = CustomCodeChoiceNo

	description := "The following changes were detected in generated SDK files:\n" + changeSummary + "\n\nWould you like to enable custom code preservation?"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[CustomCodeChoice]().
				Title("Changes detected in generated SDK files").
				Description(description).
				Options(
					huh.NewOption("Yes - Enable persistent edits", CustomCodeChoiceYes),
					huh.NewOption("No - Continue without preserving changes", CustomCodeChoiceNo),
					huh.NewOption("Don't ask again", CustomCodeChoiceDontAskAgain),
				).
				Value(&choice),
		),
	)

	if _, err := charm.NewForm(form).ExecuteForm(); err != nil {
		return CustomCodeChoiceNo, err
	}

	return choice, nil
}

// PromptForCustomCodeWithStep prompts the user via a PromptRunner (e.g., WorkflowStep).
// This allows the prompt to integrate with the workflow visualizer, pausing it while the prompt runs.
func PromptForCustomCodeWithStep(changeSummary string, runner PromptRunner) (CustomCodeChoice, error) {
	var choice CustomCodeChoice = CustomCodeChoiceNo

	description := "The following changes were detected in generated SDK files:\n" + changeSummary + "\n\nWould you like to enable custom code preservation?"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[CustomCodeChoice]().
				Title("Changes detected in generated SDK files").
				Description(description).
				Options(
					huh.NewOption("Yes - Enable persistent edits", CustomCodeChoiceYes),
					huh.NewOption("No - Continue without preserving changes", CustomCodeChoiceNo),
					huh.NewOption("Don't ask again", CustomCodeChoiceDontAskAgain),
				).
				Value(&choice),
		),
	).WithTheme(charm.GetFormTheme())

	if err := runner.RunPrompt(form); err != nil {
		return CustomCodeChoiceNo, err
	}

	return choice, nil
}
