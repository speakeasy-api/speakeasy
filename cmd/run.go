package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"

	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"go.uber.org/zap"
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
	Force            bool              `json:"force"`
	Output           string            `json:"output"`
}

var runCmd = &model.ExecutableCommand[RunFlags]{
	Usage: "run",
	Short: "generate an SDK, compile OpenAPI sources, and much more from a workflow.yaml file",
	Long: "run the workflow(s) defined in your `.speakeasy/workflow.yaml` file." + `
A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.
A full workflow is capable of running the following steps:
  - Downloading source OpenAPI documents from a URL
  - Merging multiple OpenAPI documents together
  - Overlaying OpenAPI documents with an OpenAPI overlay document
  - Generating one or many SDKs from the resulting OpenAPI document
  - Compiling the generated SDKs

` + "If `speakeasy run` is run without any arguments it will run either the first target in the workflow or the first source in the workflow if there are no other targets or sources, otherwise it will prompt you to select a target or source to run.",
	PreRun:           getMissingFlagVals,
	Run:              runFunc,
	RunInteractive:   runInteractive,
	RequiresAuth:     true,
	UsesWorkflowFile: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "target to run. specify 'all' to run all targets",
		},
		flag.StringFlag{
			Name:        "source",
			Shorthand:   "s",
			Description: "source to run. specify 'all' to run all sources",
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
		flag.BooleanFlag{
			Name:        "debug",
			Shorthand:   "d",
			Description: "enable writing debug files with broken code",
		},
		flag.StringFlag{
			Name:        "repo",
			Shorthand:   "r",
			Description: "the repository URL for the SDK, if the published (-p) flag isn't used this will be used to generate installation instructions",
		},
		flag.StringFlag{
			Name:        "repo-subdir",
			Shorthand:   "b",
			Description: "the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
		},
		flag.MapFlag{
			Name:        "repo-subdirs",
			Description: "a map from target ID to the subdirectory of the repository where the SDK is located in the repo, helps with documentation generation",
		},
		flag.BooleanFlag{
			Name:        "skip-compile",
			Description: "skip compilation when generating the SDK",
		},
		flag.BooleanFlag{
			Name:        "force",
			Description: "Force generation of SDKs even when no changes are present",
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

func getMissingFlagVals(ctx context.Context, flags *RunFlags) error {
	wf, _, err := utils.GetWorkflowAndDir()
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
			flags.Target, err = askForTarget("What target would you like to run?", "You may choose an individual target or 'all'.", "Let's configure a target for your workflow.", targets, true)
			if err != nil {
				return err
			}
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
						return nil, fmt.Errorf("%ss flag must contain an entry for target %s", flagName, target)
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

func askForTarget(title, description, confirmation string, targets []string, allowAll bool) (string, error) {
	var targetOptions []huh.Option[string]
	var existingTargets []string

	for _, targetName := range targets {
		existingTargets = append(existingTargets, targetName)
		targetOptions = append(targetOptions, huh.NewOption(targetName, targetName))
	}
	if allowAll {
		targetOptions = append(targetOptions, huh.NewOption("✱ All", "all"))
	}

	target := ""

	prompt := charm.NewSelectPrompt(title, description, targetOptions, &target)
	if _, err := charm.NewForm(huh.NewForm(prompt), confirmation).ExecuteForm(); err != nil {
		return "", err
	}

	return target, nil
}

func runFunc(ctx context.Context, flags RunFlags) error {
	workflow, err := run.NewWorkflow("Workflow", flags.Target, flags.Source, flags.Repo, flags.RepoSubdirs, flags.InstallationURLs, flags.Debug, !flags.SkipCompile, flags.Force)
	if err != nil {
		return err
	}

	err = workflow.Run(ctx)

	workflow.RootStep.Finalize(err == nil)

	addGitHubSummary(ctx, workflow)

	return err
}

func runInteractive(ctx context.Context, flags RunFlags) error {
	workflow, err := run.NewWorkflow("ignored", flags.Target, flags.Source, flags.Repo, flags.RepoSubdirs, flags.InstallationURLs, flags.Debug, !flags.SkipCompile, flags.Force)
	if err != nil {
		return err
	}

	switch flags.Output {
	case "summary":
		return workflow.RunWithVisualization(ctx)
	case "mermaid":
		err = workflow.Run(ctx)
		workflow.RootStep.Finalize(err == nil)
		mermaid, err := workflow.RootStep.ToMermaidDiagram()
		if err != nil {
			return err
		}
		log.From(ctx).Println("\n" + styles.MakeSection("Mermaid diagram of workflow", mermaid, styles.Colors.Blue))
	case "console":
		return runFunc(ctx, flags)
	}

	return nil
}

func addGitHubSummary(ctx context.Context, workflow *run.Workflow) {
	if !env.IsGithubAction() {
		return
	}

	logger := log.From(ctx)
	md := ""
	chart, err := workflow.RootStep.ToMermaidDiagram()
	if err == nil {
		md = fmt.Sprintf("# Generation Workflow Summary\n\n_This is a breakdown of the 'Generate Target' step above_\n%s", chart)
	} else {
		logger.Error("failed to generate github workflow summary", zap.Error(err))
		md = "# Generation Workflow Summary\n\n:stop_sign: Failed to generate workflow summary. Please try again or [contact support](mailto:support@speakeasyapi.dev)."
	}

	githubactions.AddStepSummary(md)
}
