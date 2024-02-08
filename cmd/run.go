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
	Target           string            `json:"target"`
	Source           string            `json:"source"`
	InstallationURL  string            `json:"installationURL"`
	InstallationURLs map[string]string `json:"installationURLs"`
	Debug            bool              `json:"debug"`
	Repo             string            `json:"repo"`
	RepoSubdir       string            `json:"repo-subdir"`
	RepoSubdirs      map[string]string `json:"repo-subdirs"`
	SkipCompile      bool              `json:"skip-compile"`
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
		model.MapFlag{
			Name:        "installationURLs",
			Description: "a map from target ID to installation URL for installation instructions if the SDK is not published to a package manager",
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
		model.MapFlag{
			Name:        "repo-subdirs",
			Description: "a map from target ID to the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
		},
		model.BooleanFlag{
			Name:        "skip-compile",
			Description: "skip compilation when generating the SDK",
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

	if flags.Target == "all" && len(targets) == 1 {
		flags.Target = targets[0]
	}

	// Gets a proper value for a mapFlag based on the singleFlag value and the mapFlag value
	// Helps ensure that the mapFlag ends up with a value for all the targets being run
	checkAndGetMapFlagValue := func(flagName, singleFlag string, mapFlag map[string]string) (map[string]string, error) {
		// If the single flag value is set, ensure we aren't running all targets, then set the map flag to the single flag value
		if singleFlag != "" && len(mapFlag) == 0 {
			if flags.Target == "all" {
				return nil, fmt.Errorf("cannot specify singular %s when running all targets. Please use the %ss flag instead", flagName, flagName)
			}

			return map[string]string{flags.Target: singleFlag}, nil
		} else if len(mapFlag) > 0 {
			// Ensure the map flag contains an entry for all targets we are running
			if flags.Target != "all" {
				if _, ok := mapFlag[flags.Target]; !ok {
					return nil, fmt.Errorf("%ss flag must contain an entry for target %s", flagName, flags.Target)
				}
			} else {
				for _, target := range targets {
					if _, ok := mapFlag[target]; !ok {
						return nil, fmt.Errorf("%ss flag must contain an entry for target %s", flagName, flags.Target)
					}
				}
			}

			return mapFlag, nil
		}

		return nil, nil
	}

	// Ensure installationURLs are properly set
	installationURLs, err := checkAndGetMapFlagValue("installationURL", flags.InstallationURL, flags.InstallationURLs)
	if err != nil {
		return err
	}
	flags.InstallationURLs = installationURLs

	// Ensure repoSubdirs are properly set
	repoSubdirs, err := checkAndGetMapFlagValue("repoSubdir", flags.RepoSubdir, flags.RepoSubdirs)
	if err != nil {
		return err
	}
	flags.RepoSubdirs = repoSubdirs

	return nil
}

func runFunc(ctx context.Context, flags RunFlags) error {
	workflow, err := run.NewWorkflow("Workflow", flags.Target, flags.Source, genVersion, flags.Repo, flags.RepoSubdirs, flags.InstallationURLs, flags.Debug, !flags.SkipCompile)
	if err != nil {
		return err
	}

	err = workflow.Run(ctx)
	if err != nil {
		return err
	}

	workflow.RootStep.Finalize(err == nil)

	if env.IsGithubAction() {
		md := fmt.Sprintf("# Generation Workflow Summary\n_This is a breakdown of the 'Generate Target' step above_\n%s", workflow.RootStep.ToMermaidDiagram())
		githubactions.AddStepSummary(md)
	}

	return err
}

func runInteractive(ctx context.Context, flags RunFlags) error {
	workflow, err := run.NewWorkflow("ignored", flags.Target, flags.Source, genVersion, flags.Repo, flags.RepoSubdirs, flags.InstallationURLs, flags.Debug, !flags.SkipCompile)
	if err != nil {
		return err
	}

	return workflow.RunWithVisualization(ctx)
}
