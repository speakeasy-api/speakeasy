package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

// FilesFlags for the files subcommand
type FilesFlags struct {
	OldSpec   string `json:"old"`
	NewSpec   string `json:"new"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
}

const diffFilesLong = `# Diff Files

Compare two OpenAPI spec files directly from disk and show SDK-level changes.

This is the simplest mode - just provide paths to your old and new spec files.

Example usage:
` + "```bash" + `
speakeasy diff files --old ./old-openapi.yaml --new ./new-openapi.yaml

# Use a specific language for SDK diff context
speakeasy diff files --old v1.yaml --new v2.yaml --lang typescript

# Specify output directory for intermediate files
speakeasy diff files --old old.json --new new.json --output-dir ./diff-output
` + "```"

var diffFilesCmd = &model.ExecutableCommand[FilesFlags]{
	Usage: "files",
	Short: "Compare two local spec files",
	Long:  utils.RenderMarkdown(diffFilesLong),
	Run:   runDiffFiles,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "old",
			Description: "Path to the old OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "new",
			Description: "Path to the new OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory for intermediate files",
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
	},
}

func runDiffFiles(ctx context.Context, flags FilesFlags) error {
	logger := log.From(ctx)

	// Validate files exist
	if _, err := os.Stat(flags.OldSpec); os.IsNotExist(err) {
		return fmt.Errorf("old spec file not found: %s", flags.OldSpec)
	}
	if _, err := os.Stat(flags.NewSpec); os.IsNotExist(err) {
		return fmt.Errorf("new spec file not found: %s", flags.NewSpec)
	}

	// Get absolute paths
	oldSpecPath, err := filepath.Abs(flags.OldSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve old spec path: %w", err)
	}
	newSpecPath, err := filepath.Abs(flags.NewSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve new spec path: %w", err)
	}

	logger.Infof("Old spec: %s", oldSpecPath)
	logger.Infof("New spec: %s", newSpecPath)
	logger.Infof("")

	return executeLocalDiff(ctx, LocalDiffParams{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
		OutputDir:   flags.OutputDir,
		Lang:        flags.Lang,
	})
}
