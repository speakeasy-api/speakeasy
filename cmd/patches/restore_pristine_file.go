package patches

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	content, err := gitRepo.GetBlob(tracked.PristineGitObject)
	if err != nil {
		return fmt.Errorf("failed to read pristine object %s: %w", tracked.PristineGitObject, err)
	}

	fullPath := filepath.Join(dir, flags.File)

	// Preserve existing file permissions
	perm := os.FileMode(0o644)
	if info, err := os.Stat(fullPath); err == nil {
		perm = info.Mode().Perm()
	}

	if err := os.WriteFile(fullPath, content, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	fmt.Printf("Restored %s to pristine version\n", flags.File)
	return nil
}
