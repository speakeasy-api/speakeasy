package patches

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	internalPatches "github.com/speakeasy-api/speakeasy/internal/patches"
)

type moveFileFlags struct {
	Dir  string `json:"dir"`
	File string `json:"file"`
	To   string `json:"to"`
}

var moveFileCmd = &model.ExecutableCommand[moveFileFlags]{
	Usage:   "mv",
	Aliases: []string{"move"},
	Short:   "Record that a tracked generated file was moved to a new path",
	Run:     runMoveFile,
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
			Description: "original tracked file path recorded in gen.lock",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "to",
			Description: "new relative path on disk for the moved file",
			Required:    true,
		},
	},
}

func runMoveFile(ctx context.Context, flags moveFileFlags) error {
	absDir, _, err := loadLockFile(flags.Dir)
	if err != nil {
		return err
	}

	if err := internalPatches.RecordMove(absDir, flags.File, flags.To); err != nil {
		return err
	}

	fmt.Printf("Recorded move: %s -> %s\n", flags.File, flags.To)
	return nil
}
