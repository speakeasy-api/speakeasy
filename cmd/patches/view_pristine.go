package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type viewPristineFlags struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
}

var viewPristineCmd = &model.ExecutableCommand[viewPristineFlags]{
	Usage: "view-pristine",
	Short: "Show the pristine (generated) version of a tracked file",
	Run:   runViewPristine,
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

func runViewPristine(ctx context.Context, flags viewPristineFlags) error {
	_, tracked, gitRepo, err := loadTrackedFile(flags.Dir, flags.File)
	if err != nil {
		return err
	}

	content, err := gitRepo.GetBlob(tracked.PristineGitObject)
	if err != nil {
		return fmt.Errorf("failed to read pristine object %s: %w", tracked.PristineGitObject, err)
	}

	fmt.Print(string(content))
	return nil
}
