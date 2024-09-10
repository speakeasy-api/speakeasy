package interactivity

import (
	"github.com/speakeasy-api/huh"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
)

func SimpleConfirm(message string, defaultValue bool) bool {
	var confirm bool = defaultValue

	if _, err := charm_internal.NewForm(
		huh.NewForm(charm_internal.NewBranchPrompt(message, &confirm)),
	).
		ExecuteForm(); err != nil {
		return false
	}

	return confirm
}

func SimpleConfirmWithOnlyAccept(message string) {
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title(message).
		Affirmative("Okay"),
	))

	if _, err := charm_internal.NewForm(form).ExecuteForm(); err != nil {
		return
	}
}
