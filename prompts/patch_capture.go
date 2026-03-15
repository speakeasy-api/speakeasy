package prompts

import (
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

func PromptForPatchCapture(changeSummary string) (bool, error) {
	choice := false

	description := "The following changes were detected in generated SDK files:\n" + changeSummary + "\n\nWould you like Speakeasy to capture them into trusted patch state before generation continues?"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Capture generated-file edits before running generation").
				Description(description).
				Affirmative("Capture now").
				Negative("Abort").
				Value(&choice),
		),
	)

	if _, err := charm.NewForm(form).ExecuteForm(); err != nil {
		return false, err
	}

	return choice, nil
}

func PromptForPatchCaptureWithStep(changeSummary string, runner PromptRunner) (bool, error) {
	choice := false

	description := "The following changes were detected in generated SDK files:\n" + changeSummary + "\n\nWould you like Speakeasy to capture them into trusted patch state before generation continues?"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Capture generated-file edits before running generation").
				Description(description).
				Affirmative("Capture now").
				Negative("Abort").
				Value(&choice),
		),
	).WithTheme(charm.GetFormTheme())

	if err := runner.RunPrompt(form); err != nil {
		return false, err
	}

	return choice, nil
}
