package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	internalPatches "github.com/speakeasy-api/speakeasy/internal/patches"
)

type restorePristineAllFlags struct {
	Dir string `json:"dir"`
}

var restorePristineAllCmd = &model.ExecutableCommand[restorePristineAllFlags]{
	Usage: "all",
	Short: "Restore all files with custom code to their pristine (generated) versions",
	Run:   runRestorePristineAll,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "project directory containing .speakeasy/gen.lock",
			DefaultValue: ".",
		},
	},
}

func runRestorePristineAll(ctx context.Context, flags restorePristineAllFlags) error {
	dir, lf, err := loadLockFile(flags.Dir)
	if err != nil {
		return err
	}

	gitRepo, err := internalPatches.OpenGitRepository(dir)
	if err != nil {
		return err
	}

	var restored int
	for path := range lf.TrackedFiles.Keys() {
		tracked, ok := lf.TrackedFiles.Get(path)
		if !ok || tracked.PristineGitObject == "" {
			continue
		}

		pristine, err := gitRepo.GetBlob(tracked.PristineGitObject)
		if err != nil {
			continue
		}

		if fileMatchesPristine(dir, path, pristine) {
			continue
		}

		if err := restoreFileToPristine(dir, path, pristine); err != nil {
			return err
		}

		fmt.Printf("  Restored %s\n", path)
		restored++
	}

	if restored == 0 {
		fmt.Println("No files with custom code detected.")
	} else {
		fmt.Printf("Restored %d file(s) to pristine version\n", restored)
	}

	return nil
}
