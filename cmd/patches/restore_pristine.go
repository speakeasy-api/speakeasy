package patches

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
)

var restorePristineCmd = &model.CommandGroup{
	Usage:    "restore-pristine",
	Short:    "Restore files to their pristine (generated) version, discarding custom edits",
	Commands: []model.Command{restorePristineAllCmd, restorePristineFileCmd},
}
