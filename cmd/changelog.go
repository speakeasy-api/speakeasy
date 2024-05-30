package cmd

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/changelogs"
	genConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/run"
)

var changelogCmd = &model.CommandGroup{
	Usage:          "changelog",
	Short:          "Utilities for working with changelogs",
	Long:           `The "changelog" command provides commands for creating and manipulating changelogs`,
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{generateChangelogCmd},
}

type generateChangelogFlags struct {
	Target string `json:"target"`
	Out    string `json:"out"`
	Format string `json:"format"`
}

var generateChangelogCmd = &model.ExecutableCommand[generateChangelogFlags]{
	Usage: "generate",
	Short: "Generate a changelog for a given target, without regenerating that target",
	RequiresAuth:     true,
	UsesWorkflowFile: true,
	Run:            generateChangelog,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "target to run.",
		},
		flag.StringFlag{
			Name:        "out",
			Shorthand:   "o",
			Description: "write directly to a file instead of stdout",
		},
		flag.EnumFlag{
			Name:          "format",
			Shorthand:     "f",
			Description:   "output format",
			DefaultValue: "markdown",
			AllowedValues: []string{"json", "markdown"},
		},
	},
}

func generateChangelog(ctx context.Context, flags generateChangelogFlags) error {
	workflow, err := run.NewWorkflow(
		ctx,
		"Workflow",
		flags.Target,
		"",
		"",
		nil,
		nil,
		false,
		false,
		false,
		[]string{},
	)
	if err != nil {
		return err
	}

	target, err := workflow.ValidateSingleTarget()
	if err != nil {
		return err
	}

	_, result, err := workflow.RunSource(ctx, workflow.RootStep, target.Source, target.Target, true)
	if err != nil {
		return err
	}
	summary, err := result.Changes.GetSummary()
	if err != nil {
		return err
	}
	var outDir string
	if target.Output != nil {
		outDir = *target.Output
	} else {
		outDir = workflow.ProjectDir
	}

	fmt.Printf("Old: %s\nNew %s\n# Summary\n%s\n", result.OldRevision, result.NewRevision, summary.Text)

	lang := target.Target

	latestVersions, err := changelogs.GetLatestVersions(lang)
	if err != nil {
		return fmt.Errorf("failed to get latest versions for language %s: %w", lang, err)
	}
	lockFile, err := genConfig.Load(outDir)
	if err != nil {
		return err
	}

	features, ok := lockFile.LockFile.Features[lang]
	if !ok {
		features = nil
	}

	changelogStr, err := changelogs.GetChangeLog(lang, latestVersions, features)
	if err != nil {
		return err
	}

	fmt.Printf(changelogStr)

	return nil
}
