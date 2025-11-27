package prompts

import (
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

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
