package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	internalPatches "github.com/speakeasy-api/speakeasy/internal/patches"
)

type viewDiffFilesFlags struct {
	Dir string `json:"dir"`
}

var viewDiffFilesCmd = &model.ExecutableCommand[viewDiffFilesFlags]{
	Usage: "files",
	Short: "List files that have custom code applied (differ from pristine generated version)",
	Run:   runViewDiffFiles,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "dir",
			Shorthand:    "d",
			Description:  "project directory containing .speakeasy/gen.lock",
			DefaultValue: ".",
		},
	},
}

func runViewDiffFiles(ctx context.Context, flags viewDiffFilesFlags) error {
	dir, lf, err := loadLockFile(flags.Dir)
	if err != nil {
		return err
	}

	gitRepo, err := internalPatches.OpenGitRepository(dir)
	if err != nil {
		return err
	}

	// Compare every tracked file on disk against its pristine git object.
	// "Custom code" = file exists on disk AND differs from the pristine (generated) version.
	var diffs []internalPatches.FileDiff
	for path := range lf.TrackedFiles.Keys() {
		tracked, ok := lf.TrackedFiles.Get(path)
		if !ok || tracked.PristineGitObject == "" {
			continue
		}

		fd := internalPatches.ComputeFileDiff(dir, path, tracked.PristineGitObject, gitRepo)
		if fd.Stats.Added+fd.Stats.Removed > 0 {
			diffs = append(diffs, fd)
		}
	}

	if len(diffs) == 0 {
		fmt.Println("No files with custom code detected.")
		return nil
	}

	fmt.Println("Files with custom code:")
	for _, fd := range diffs {
		fmt.Printf("  M %s (+%d/-%d)\n", fd.Path, fd.Stats.Added, fd.Stats.Removed)
	}

	return nil
}
