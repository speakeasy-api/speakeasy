package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/studio"
	"github.com/spf13/cobra"

	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/run"
)

type RunFlags struct {
	Target             string            `json:"target"`
	Source             string            `json:"source"`
	InstallationURL    string            `json:"installationURL"`
	InstallationURLs   map[string]string `json:"installationURLs"`
	Debug              bool              `json:"debug"`
	Repo               string            `json:"repo"`
	RepoSubdir         string            `json:"repo-subdir"`
	RepoSubdirs        map[string]string `json:"repo-subdirs"`
	SkipCompile        bool              `json:"skip-compile"`
	SkipTesting        bool              `json:"skip-testing"`
	SkipVersioning     bool              `json:"skip-versioning"`
	SkipUploadSpec     bool              `json:"skip-upload-spec"`
	FrozenWorkflowLock bool              `json:"frozen-workflow-lockfile"`
	Force              bool              `json:"force"`
	Output             string            `json:"output"`
	Pinned             bool              `json:"pinned"`
	Verbose            bool              `json:"verbose"`
	RegistryTags       []string          `json:"registry-tags"`
	SetVersion         string            `json:"set-version"`
	Watch              bool              `json:"watch"`
	GitHub             bool              `json:"github"`
	GitHubRepos        string            `json:"github-repos"`
	Minimal            bool              `json:"minimal"`
}

const runLong = "# Run \n Execute the workflow(s) defined in your `.speakeasy/workflow.yaml` file." + `

A workflow can consist of multiple targets that define a source OpenAPI document that can be downloaded from a URL, exist as a local file, or be created via merging multiple OpenAPI documents together and/or overlaying them with an OpenAPI overlay document.

A full workflow is capable of running the following:
  - Downloading source OpenAPI documents from a URL
  - Merging multiple OpenAPI documents together
  - Overlaying OpenAPI documents with an OpenAPI overlay document
  - Generating one or many SDKs from the resulting OpenAPI document
  - Compiling the generated SDKs

` + "If `speakeasy run` is run without any arguments it will run either the first target in the workflow or the first source in the workflow if there are no other targets or sources, otherwise it will prompt you to select a target or source to run."

var runCmd = &model.ExecutableCommand[RunFlags]{
	Usage:            "run",
	Short:            "Run all the workflows defined in your workflow.yaml file. This can include multiple SDK generations from different OpenAPI sources",
	Long:             utils.RenderMarkdown(runLong),
	PreRun:           preRun,
	Run:              runNonInteractive,
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
			Name:        "skip-testing",
			Description: "skip testing after generating the SDK, if testing is configured in the workflow",
		},
		flag.BooleanFlag{
			Name:         "skip-versioning",
			Description:  "skip automatic SDK version increments",
			DefaultValue: false,
		},
		flag.BooleanFlag{
			Name:        "skip-upload-spec",
			Description: "skip uploading the spec to the registry",
		},
		flag.BooleanFlag{
			Name:         "frozen-workflow-lockfile",
			Description:  "executes using the stored inputs from the workflow.lock, such that no OAS change occurs",
			DefaultValue: false,
			Hidden:       true, // we are unaware of any use cases for this flag outside of upgrade regression testing, which we execute internally
		},
		flag.BooleanFlag{
			Name:               "force",
			Description:        "Force generation of SDKs even when no changes are present",
			Deprecated:         true,
			DeprecationMessage: "as it is now the default behavior and will be removed in a future version",
		},
		flag.EnumFlag{
			Name:          "output",
			Shorthand:     "o",
			Description:   "What to output while running",
			AllowedValues: []string{"summary", "mermaid", "console"},
			DefaultValue:  "summary",
		},
		flag.BooleanFlag{
			Name:        "pinned",
			Description: "Run using the current CLI version instead of the version specified in the workflow file",
			Hidden:      true,
		},
		flag.BooleanFlag{
			Name:        "verbose",
			Description: "Verbose logging",
			Hidden:      false,
		},
		flag.StringSliceFlag{
			Name:        "registry-tags",
			Description: "tags to apply to the speakeasy registry bundle",
		},
		flag.StringFlag{
			Name:        "set-version",
			Description: "the manual version to apply to the generated SDK",
		},
		flag.BooleanFlag{
			Name:        "watch",
			Shorthand:   "w",
			Description: "launch the web studio for improving the quality of the generated SDK",
			Required:    false,
		},
		flag.BooleanFlag{
			Name:        "github",
			Description: "kick off a generation run in GitHub for the repository pertaining to your current directory",
		},
		flag.StringFlag{
			Name:        "github-repos",
			Description: "GLOBAL: run SDK generation across your entire Speakeasy workspace/account, independent of your current directory. Use 'all' for all connected repos or a comma-separated list of GitHub repo URLs",
		},
		flag.BooleanFlag{
			Name:        "minimal",
			Description: "only run the steps that are strictly necessary to generate the SDK",
		},
	},
}

// Gets missing flag values (ie source / target)
func preRun(cmd *cobra.Command, flags *RunFlags) error {
	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	sources, targets, err := run.ParseSourcesAndTargets()
	if err != nil {
		return err
	}

	if flags.GitHubRepos != "" {
		flags.GitHub = true
		if err := cmd.Flags().Set("github", "true"); err != nil {
			return err
		}
	}

	if flags.Target == "" && flags.Source == "" {
		if len(wf.Targets) == 1 {
			flags.Target = targets[0]
		} else if len(wf.Targets) == 0 && len(wf.Sources) == 1 {
			flags.Source = sources[0]
		} else if len(wf.Targets) == 0 && len(wf.Sources) > 1 {
			flags.Source, err = askForSource(sources)
			if err != nil {
				return err
			}
		} else {
			flags.Target, err = askForTarget("What target would you like to run?", "You may choose an individual target or 'all'.", "Let's choose a target to run the generation workflow.", targets, true)
			if err != nil {
				return err
			}
		}
	}

	if flags.Target == "all" && len(targets) == 1 {
		flags.Target = targets[0]
	}

	// Needed later
	if err := cmd.Flags().Set("target", flags.Target); err != nil {
		return err
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
	if _, err := charm.NewForm(huh.NewForm(prompt), charm.WithTitle(confirmation)).ExecuteForm(); err != nil {
		return "", err
	}

	return target, nil
}

func askForSource(sources []string) (string, error) {
	var sourceOptions []huh.Option[string]

	for _, sourceName := range sources {
		sourceOptions = append(sourceOptions, huh.NewOption(sourceName, sourceName))
	}

	sourceOptions = append(sourceOptions, huh.NewOption("✱ All", "all"))

	source := ""

	prompt := charm.NewSelectPrompt("What source would you like to run?", "You may choose an individual target or 'all'.", sourceOptions, &source)
	if _, err := charm.NewForm(huh.NewForm(prompt), charm.WithTitle("Let's choose a target to run the generation workflow.")).ExecuteForm(); err != nil {
		return "", err
	}

	return source, nil
}

var minimalOpts = []run.Opt{
	run.WithSkipChangeReport(true),
	run.WithSkipSnapshot(true),
	run.WithSkipTesting(true),
	run.WithSkipGenerateLintReport(),
}

func runNonInteractive(ctx context.Context, flags RunFlags) error {
	if flags.GitHub {
		if flags.GitHubRepos != "" {
			return run.RunGitHubRepos(ctx, flags.Target, flags.SetVersion, flags.Force, flags.GitHubRepos)
		}
		return run.RunGitHub(ctx, flags.Target, flags.SetVersion, flags.Force)
	}

	opts := []run.Opt{
		run.WithTarget(flags.Target),
		run.WithSource(flags.Source),
		run.WithRepo(flags.Repo),
		run.WithRepoSubDirs(flags.RepoSubdirs),
		run.WithInstallationURLs(flags.InstallationURLs),
		run.WithDebug(flags.Debug),
		run.WithShouldCompile(!flags.SkipCompile),
		run.WithSkipTesting(flags.SkipTesting),
		run.WithSkipVersioning(flags.SkipVersioning),
		run.WithSkipSnapshot(flags.SkipUploadSpec),
		run.WithVerbose(flags.Verbose),
		run.WithRegistryTags(flags.RegistryTags),
		run.WithSetVersion(flags.SetVersion),
		run.WithFrozenWorkflowLock(flags.FrozenWorkflowLock),
		run.WithSkipCleanup(), // The studio won't work if we clean up before it launches
	}

	if flags.Minimal {
		opts = append(opts, minimalOpts...)
	}

	workflow, err := run.NewWorkflow(
		ctx,
		opts...,
	)

	if err != nil {
		return err
	}

	err = workflow.Run(ctx)

	defer func() {
		// we should leave temp directories for debugging if run fails
		if err == nil || env.IsGithubAction() {
			workflow.Cleanup()
		}
	}()

	// We don't return the error here because we want to try to launch the studio to help fix the issue, if possible
	workflow.RootStep.Finalize(err == nil)

	github.GenerateWorkflowSummary(ctx, workflow.RootStep)

	if studioErr, studioLaunched := maybeLaunchStudio(ctx, workflow, flags, err); !studioLaunched {
		return err // Now return the original error if we didn't launch the studio
	} else {
		return studioErr
	}
}

func runInteractive(ctx context.Context, flags RunFlags) error {
	if flags.GitHub {
		if flags.GitHubRepos != "" {
			return run.RunGitHubRepos(ctx, flags.Target, flags.SetVersion, flags.Force, flags.GitHubRepos)
		}
		return run.RunGitHub(ctx, flags.Target, flags.SetVersion, flags.Force)
	}

	opts := []run.Opt{
		run.WithTarget(flags.Target),
		run.WithSource(flags.Source),
		run.WithSkipTesting(flags.SkipTesting),
		run.WithSkipVersioning(flags.SkipVersioning),
		run.WithRepo(flags.Repo),
		run.WithRepoSubDirs(flags.RepoSubdirs),
		run.WithInstallationURLs(flags.InstallationURLs),
		run.WithDebug(flags.Debug),
		run.WithShouldCompile(!flags.SkipCompile),
		run.WithVerbose(flags.Verbose),
		run.WithRegistryTags(flags.RegistryTags),
		run.WithSetVersion(flags.SetVersion),
		run.WithFrozenWorkflowLock(flags.FrozenWorkflowLock),
		run.WithSkipCleanup(), // The studio won't work if we clean up before it launches
	}

	if flags.Minimal {
		opts = append(opts, minimalOpts...)
	}

	workflow, err := run.NewWorkflow(
		ctx,
		opts...,
	)

	if err != nil {
		return err
	}

	if flags.Verbose {
		flags.Output = "console"
	}

	switch flags.Output {
	case "summary":
		err = workflow.RunWithVisualization(ctx)
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
		workflow.RootStep.Finalize(err == nil)
	}

	defer func() {
		// we should leave temp directories for debugging if run fails
		if err == nil || env.IsGithubAction() {
			workflow.Cleanup()
		}
	}()

	// We don't return the error here because we want to try to launch the studio to help fix the issue, if possible
	if err == nil {
		workflow.PrintSuccessSummary(ctx)
	}

	if studioErr, studioLaunched := maybeLaunchStudio(ctx, workflow, flags, err); !studioLaunched {
		return err // Now return the original error if we didn't launch the studio
	} else {
		return studioErr
	}
}

// We'll only print the runErr if we actually launch the studio. Otherwise, it will get printed when we return all the way out
func maybeLaunchStudio(ctx context.Context, wf *run.Workflow, flags RunFlags, runErr error) (error, bool) {
	if studio.CanLaunch(ctx, wf) && flags.Watch {
		if runErr != nil {
			log.From(ctx).Error(runErr.Error())
		}
		return studio.LaunchStudio(ctx, wf), true
	} else if wf.CountDiagnostics() > 1 {
		log.From(ctx).PrintfStyled(styles.Info, "\nWe've detected `%d` potential improvements for your SDK.\nGet automatic fixes in the Studio with `speakeasy run --watch`", wf.CountDiagnostics())
	} else if wf.CountDiagnostics() == 1 {
		log.From(ctx).PrintfStyled(styles.Info, "\nWe've detected `1` potential improvement for your SDK.\nGet automatic fixes in the Studio with `speakeasy run --watch`")
	}

	return nil, false
}
