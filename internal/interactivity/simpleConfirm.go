package interactivity

import (
	"github.com/speakeasy-api/huh"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
)

func SimpleConfirm(message string) bool {
	var confirm bool

	if _, err := charm_internal.NewForm(
		huh.NewForm(charm_internal.NewBranchPrompt(message, &confirm)),
	).
		ExecuteForm(); err != nil {
		return false
	}

	return confirm
}
