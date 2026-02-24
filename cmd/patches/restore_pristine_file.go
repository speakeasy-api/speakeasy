package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type restorePristineFileFlags struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
}

var restorePristineFileCmd = &model.ExecutableCommand[restorePristineFileFlags]{
	Usage: "file",
	Short: "Restore a file to its pristine (generated) version, discarding custom edits",
	Run:   runRestorePristineFile,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "project directory containing .speakeasy/gen.lock",
			DefaultValue: ".",
		},
		flag.StringFlag{
			Name:        "file",
			Shorthand:   "f",
			Description: "relative path to the tracked file (e.g. src/sdk/foo.py)",
			Required:    true,
		},
	},
}

func runRestorePristineFile(ctx context.Context, flags restorePristineFileFlags) error {
	dir, tracked, gitRepo, err := loadTrackedFile(flags.Dir, flags.File)
	if err != nil {
		return err
	}

	if err := restoreFileToPristine(dir, flags.File, gitRepo, tracked); err != nil {
		return err
	}

	fmt.Printf("Restored %s to pristine version\n", flags.File)
	return nil
}
