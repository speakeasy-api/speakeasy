package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"

	"github.com/speakeasy-api/speakeasy/internal/model"
)

type CleanFlags struct {
	Global bool `json:"global"`
}

var cleanCmd = &model.ExecutableCommand[CleanFlags]{
	Usage: "clean",
	Short: "Speakeasy clean can be used to clean up cache, stale temp folders, and old CLI binaries.",
	Long: `Using speakeasy clean outside of an SDK directory or with the --global will clean cache, CLI binaries, and more out of the root .speakeasy folder.
Within an SDK directory, it will clean out stale entries within the local .speakeasy folder.`,
	Run: cleanExec,
	Flags: []flag.Flag{
		flag.BooleanFlag{
			Name:        "global",
			Description: "clean out the root .speakeasy directory",
		},
	},
}

func cleanExec(ctx context.Context, flags CleanFlags) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	var cleanedFolder string
	if workflowFile, path, _ := workflow.Load(workingDir); !flags.Global && workflowFile != nil && path != "" {
		localPath := strings.TrimSuffix(path, "/workflow.yaml")
		cleanedFolder = localPath
		if _, err := os.Stat(filepath.Join(localPath, "temp")); err == nil {
			err = os.RemoveAll(filepath.Join(localPath, "temp"))
			if err != nil {
				return err
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		cfgDir := filepath.Join(home, ".speakeasy")
		cleanedFolder = cfgDir
		if _, err := os.Stat(filepath.Join(cfgDir, "temp")); err == nil {
			err = os.RemoveAll(filepath.Join(cfgDir, "temp"))
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(filepath.Join(cfgDir, "cache")); err == nil {
			err = os.RemoveAll(filepath.Join(cfgDir, "cache"))
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(filepath.Join(cfgDir, ".log.events.json")); err == nil {
			err = os.RemoveAll(filepath.Join(cfgDir, ".log.events.json"))
			if err != nil {
				return err
			}
		}

		// remove stale CLI binaries
		files, err := os.ReadDir(cfgDir)
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() && strings.HasPrefix(file.Name(), "1.") {
				err = os.RemoveAll(filepath.Join(cfgDir, file.Name()))
				if err != nil {
					return err
				}
			}
		}
	}

	fmt.Println(
		styles.RenderSuccessMessage(
			fmt.Sprintf("Speakeasy directory at path %s successfully cleaned!", cleanedFolder),
		),
	)
	return nil
}
