package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	internalPatches "github.com/speakeasy-api/speakeasy/internal/patches"
)

type viewDiffFileFlags struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
}

var viewDiffFileCmd = &model.ExecutableCommand[viewDiffFileFlags]{
	Usage: "file",
	Short: "Show the diff between pristine (generated) and current version of a file",
	Run:   runViewDiffFile,
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

func runViewDiffFile(ctx context.Context, flags viewDiffFileFlags) error {
	dir, tracked, gitRepo, err := loadTrackedFile(flags.Dir, flags.File)
	if err != nil {
		return err
	}

	fd := internalPatches.ComputeFileDiff(dir, flags.File, tracked.PristineGitObject, gitRepo)

	if fd.Stats.Added+fd.Stats.Removed == 0 {
		fmt.Println("No custom code detected in this file.")
		return nil
	}

	fmt.Print(fd.DiffText)

	return nil
}
