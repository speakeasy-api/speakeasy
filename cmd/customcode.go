package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/registercustomcode"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/env"

	"github.com/speakeasy-api/speakeasy/internal/run"
)

type RegisterCustomCodeFlags struct {
	Show    bool   `json:"show"`
	Resolve bool   `json:"resolve"`
	Apply bool   `json:"apply-only"`
	ApplyReverse bool	`json:"apply-reverse"`
	LatestHash bool   `json:"latest-hash"`
	InstallationURL	string	`json:"installationURL"`
	InstallationURLs   map[string]string `json:"installationURLs"`
	Repo               string            `json:"repo"`
	RepoSubdir         string            `json:"repo-subdir"`
	RepoSubdirs        map[string]string `json:"repo-subdirs"`
	SkipVersioning     bool              `json:"skip-versioning"`
	Output             string            `json:"output"`
	SetVersion         string            `json:"set-version"`

}

var registerCustomCodeCmd = &model.ExecutableCommand[RegisterCustomCodeFlags]{
	Usage:  "customcode",
	Short:  "Register custom code with the OpenAPI generation system.",
	Long:   `Register custom code with the OpenAPI generation system.`,
	Run:    registerCustomCode,
	Flags:  []flag.Flag{
		flag.BooleanFlag{
			Name:        "show",
			Shorthand:   "s",
			Description: "show custom code patches",
		},
		flag.BooleanFlag{
			Name:        "apply-only",
			Shorthand:   "a",
			Description: "apply existing custom code patches without running generation",
		},
		flag.BooleanFlag{
			Name:		 "latest-hash",
			Description: "show the latest commit hash from gen.lock that contains custom code changes",
		},
		flag.StringFlag{
			Name:        "installationURL",
			Shorthand:   "i",
			Description: "the language specific installation URL for installation instructions if the SDK is not published to a package manager",
		},
		flag.MapFlag{
			Name:        "installationURLs",
			Description: "a map from target ID to installation URL for installation instructions if the SDK is not published to a package manager",
		},
		flag.StringFlag{
			Name:        "repo",
			Shorthand:   "r",
			Description: "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions",
		},
		flag.EnumFlag{
			Name:          "output",
			Shorthand:     "o",
			Description:   "What to output while running",
			AllowedValues: []string{"summary", "mermaid", "console"},
			DefaultValue:  "summary",
		},
	},
}

func registerCustomCode(ctx context.Context, flags RegisterCustomCodeFlags) error {

	// If --show flag is provided, show existing customcode
	if flags.Show {
		return registercustomcode.ShowCustomCodePatch(ctx)
	}

	// If --apply flag is provided, only apply existing patches
	if flags.Apply {
		wf, _, err := utils.GetWorkflowAndDir()
		if err != nil {
			return fmt.Errorf("Could not find workflow file")
		}
		for _, target := range wf.Targets {
			return registercustomcode.ApplyCustomCodePatch(ctx, target)
		}
	}

	// If --latest-hash flag is provided, show the commit hash from gen.lock
	if flags.LatestHash {
		return registercustomcode.ShowLatestCommitHash(ctx)
	}

	// Call the registercustomcode functionality
	return registercustomcode.RegisterCustomCode(ctx, func(targetName string) error {
		opts := []run.Opt{
			run.WithTarget(targetName),
			run.WithRepo(flags.Repo),
			run.WithRepoSubDirs(flags.RepoSubdirs),
			run.WithInstallationURLs(flags.InstallationURLs),
			run.WithSkipVersioning(true),
			run.WithSkipApplyCustomCode(),
		}
		workflow, err := run.NewWorkflow(
			ctx,
			opts...,
		)
		defer func() {
			// we should leave temp directories for debugging if run fails
			if env.IsGithubAction() {
				workflow.Cleanup()
			}
		}()

		switch flags.Output {
			case "summary":
				err = workflow.RunWithVisualization(ctx)
				if err != nil {
					return err
				}
			case "mermaid":
				err = workflow.Run(ctx)
				workflow.RootStep.Finalize(err == nil)
				mermaid, err := workflow.RootStep.ToMermaidDiagram()
				if err != nil {
					return err
				}
				log.From(ctx).Println("\n" + styles.MakeSection("Mermaid diagram of workflow", mermaid, styles.Colors.Blue))
			case "console":
				err = workflow.Run(ctx)
				// workflow.RootStep.Finalize(err == nil)
				if err != nil {
					return err
				}
		}
		return nil
	}, )

}