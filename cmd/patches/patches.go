package patches

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
)

var PatchesCmd = &model.CommandGroup{
	Usage:    "patches",
	Short:    "Debug and inspect pristine vs patched SDK files",
	Commands: []model.Command{viewPristineCmd, viewDiffCmd, restorePristineCmd},
}
