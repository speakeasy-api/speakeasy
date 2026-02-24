package patches

import (
	"github.com/speakeasy-api/speakeasy/internal/model"
)

var viewDiffCmd = &model.CommandGroup{
	Usage:    "view-diff",
	Short:    "View diffs between pristine (generated) and current SDK files",
	Commands: []model.Command{viewDiffFileCmd, viewDiffFilesCmd},
}
