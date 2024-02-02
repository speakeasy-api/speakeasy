package cmd

import (
	"context"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"golang.org/x/exp/slices"
	"strings"
)

type RunFlags struct {
	Target          string `json:"target"`
	Source          string `json:"source"`
	InstallationURL string `json:"installationURL"`
	Debug           bool   `json:"debug"`
	Repo            string `json:"repo"`
	RepoSubdir      string `json:"repo-subdir"`
	Published       bool   `json:"published"`
}

var runCmd = &model.ExecutableCommand[RunFlags]{
	Usage: "run",
	Short: "run the workflow(s) defined in your `.speakeasy/workflow.yaml` file.",
	Long: "run the workflow(s) defined in your `.speakeasy/workflow.yaml` file." + `
A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.
A full workflow is capable of running the following steps:
  - Downloading source OpenAPI documents from a URL
  - Merging multiple OpenAPI documents together
  - Overlaying OpenAPI documents with an OpenAPI overlay document
  - Generating one or many SDKs from the resulting OpenAPI document
  - Compiling the generated SDKs

` + "If `speakeasy run` is run without any arguments it will run either the first target in the workflow or the first source in the workflow if there are no other targets or sources, otherwise it will prompt you to select a target or source to run.",
	PreRun:         getMissingFlagVals,
	Run:            runFunc,
	RunInteractive: runInteractive,
	RequiresAuth:   true,
	Flags: []model.Flag{
		model.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "target to run. specify 'all' to run all targets",
		},
		model.StringFlag{
			Name:        "source",
			Shorthand:   "s",
			Description: "source to run. specify 'all' to run all sources",
		},
		model.StringFlag{
			Name:        "installationURL",
			Shorthand:   "i",
			Description: "the language specific installation URL for installation instructions if the SDK is not published to a package manager",
		},
		model.BooleanFlag{
			Name:        "debug",
			Shorthand:   "d",
			Description: "enable writing debug files with broken code",
		},
		model.StringFlag{
			Name:        "repo",
			Shorthand:   "r",
			Description: "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions",
		},
		model.StringFlag{
			Name:        "repo-subdir",
			Shorthand:   "b",
			Description: "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
		},
		model.BooleanFlag{
			Name:        "published",
			Shorthand:   "p",
			Description: "whether the SDK is published to a package manager or not, determines the type of installation instructions to generate",
		},
	},
}

func getMissingFlagVals(ctx context.Context, flags *RunFlags) error {
	wf, _, err := run.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	sources, targets, err := run.ParseSourcesAndTargets()
	if err != nil {
		return err
	}

	if flags.Target == "" && flags.Source == "" {
		if len(wf.Targets) == 1 {
			flags.Target = targets[0]
		} else if len(wf.Targets) == 0 && len(wf.Sources) == 1 {
			flags.Source = sources[0]
		} else {
			// TODO update to use our proper interactive code
			prompt := promptui.Prompt{
				Label: fmt.Sprintf("Select a target (%s or 'all')", strings.Join(targets, ", ")),
				Validate: func(input string) error {
					if input == "" {
						return fmt.Errorf("target cannot be empty")
					}

					if input != "all" && !slices.Contains(targets, input) {
						return fmt.Errorf("invalid target")
					}

					return nil
				},
			}

			result, err := prompt.Run()
			if err != nil {
				return err
			}

			flags.Target = result
		}
	}

	return nil
}

func runFunc(ctx context.Context, flags RunFlags) error {
	workflow := run.NewWorkflowStep("Workflow", nil)

	err := run.Run(ctx, flags.Target, flags.Source, genVersion, flags.InstallationURL, flags.Repo, flags.RepoSubdir, flags.Debug, workflow)

	workflow.Finalize(err == nil)

	if env.IsGithubAction() {
		githubactions.AddStepSummary(workflow.ToMermaidDiagram())
	}

	return err
}

func runInteractive(ctx context.Context, flags RunFlags) error {
	return run.RunWithVisualization(ctx, flags.Target, flags.Source, genVersion, flags.InstallationURL, flags.Repo, flags.RepoSubdir, flags.Debug)
}
