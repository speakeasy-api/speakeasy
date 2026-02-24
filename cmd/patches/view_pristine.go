package patches

import (
	"context"
	"fmt"
	"path/filepath"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/git"
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
	dir, err := filepath.Abs(flags.Dir)
	if err != nil {
		return fmt.Errorf("failed to resolve directory: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", dir, err)
	}
	if cfg.LockFile == nil {
		return fmt.Errorf("no gen.lock found in %s", dir)
	}

	tracked, ok := cfg.LockFile.TrackedFiles.Get(flags.File)
	if !ok {
		return fmt.Errorf("file %q is not tracked in gen.lock", flags.File)
	}

	if tracked.PristineGitObject == "" {
		return fmt.Errorf("file %q has no pristine git object recorded", flags.File)
	}

	repo, err := git.NewLocalRepository(dir)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	if repo.IsNil() {
		return fmt.Errorf("no git repository found at %s", dir)
	}

	content, err := repo.GetBlob(tracked.PristineGitObject)
	if err != nil {
		return fmt.Errorf("failed to read pristine object %s: %w", tracked.PristineGitObject, err)
	}

	fmt.Print(string(content))
	return nil
}
