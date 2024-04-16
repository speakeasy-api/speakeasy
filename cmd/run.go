package cmd

import (
	"context"
	"fmt"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	PreRun:           preRun,
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

// Gets missing flag values (ie source / target)
// Then runs the command with the version from the workflow file
func preRun(cmd *cobra.Command, flags *RunFlags) error {
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

	return runWithVersionFromWorkflowFile(cmd, flags)
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

// If the command is run from a workflow file, check if the desired version is different from the current version
// If so, download the desired version and run the command with it as a subprocess
func runWithVersionFromWorkflowFile(cmd *cobra.Command, flags *RunFlags) error {
	ctx := cmd.Context()
	logger := log.From(ctx)

	wf, wfPath, err := utils.GetWorkflow()
	if err != nil {
		return fmt.Errorf("failed to load workflow file: %w", err)
	}

	// If the workflow file doesn't exist, or we're running locally, simply run the command normally with the existing version of the CLI
	if wf == nil { // TODO: uncomment when done testing locally || env.IsLocalDev() {
		return nil
	}

	currentlyRunningVersion := events.GetSpeakeasyVersionFromContext(ctx)
	artifactArch := ctx.Value(updates.ArtifactArchContextKey).(string)

	// Try to migrate existing workflows
	if wf.SpeakeasyVersion == "" {
		if ghPinned := env.PinnedVersion(); ghPinned != "" {
			wf.SpeakeasyVersion = workflow.Version(ghPinned)
		} else {
			wf.SpeakeasyVersion = "latest"
		}

		_ = updateWorkflowFile(wf, wfPath)
	}

	// Get the latest version, or use the pinned version
	desiredVersion := wf.SpeakeasyVersion.String()
	if desiredVersion == "latest" {
		latest, err := updates.GetLatestVersion(artifactArch)
		if err != nil {
			return err
		}
		desiredVersion = latest.String()

		logger.PrintfStyled(styles.DimmedItalic, "Running with latest Speakeasy version: %s\n", desiredVersion)
	} else {
		logger.PrintfStyled(styles.DimmedItalic, "Running with speakeasyVersion from workflow.yaml: %s\n", desiredVersion)
	}

	// If the desired version is the same as the currently running version of the CLI, just run the command
	if desiredVersion == currentlyRunningVersion {
		return nil
	}

	runErr := runWithVersion(cmd, artifactArch, desiredVersion)
	if runErr != nil {
		// If the command failed to run with the latest version, try to run with the version from the lock file
		if wf.SpeakeasyVersion == "latest" {
			msg := fmt.Sprintf("Failed to run with Speakeasy version %s: %s\n", desiredVersion, runErr.Error())
			_ = log.SendToLogProxy(ctx, log.LogProxyLevelError, msg, nil)
			logger.PrintfStyled(styles.DimmedItalic, msg)
			if env.IsGithubAction() {
				githubactions.AddStepSummary("# Speakeasy Version upgrade failure\n" + msg)
			}

			lockfileVersion := getLockfileVersion(wf, wfPath, flags.Target)
			if lockfileVersion != "" {
				logger.PrintfStyled(styles.DimmedItalic, "Rerunning with previous successful version: %s\n", lockfileVersion)
				if err := runWithVersion(cmd, artifactArch, lockfileVersion); err != nil {
					return err
				}
			}
		}

		// If the command failed to run with the pinned version, fail normally
		return runErr
	}

	// Exit to prevent the command from running twice
	os.Exit(0)
	return nil
}

func runWithVersion(cmd *cobra.Command, artifactArch, desiredVersion string) error {
	vLocation, err := updates.InstallVersion(cmd.Context(), desiredVersion, artifactArch, 30)
	if err != nil {
		return err
	}

	cmdString := utils.GetFullCommandString(cmd)
	cmdString = strings.TrimPrefix(cmdString, "speakeasy ")

	newCmd := exec.Command(vLocation, strings.Split(cmdString, " ")...)
	newCmd.Stdin = os.Stdin
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	if err = newCmd.Run(); err != nil {
		return fmt.Errorf("failed to run with version %s: %w", desiredVersion, err)
	}

	return nil
}

func getLockfileVersion(wf *workflow.Workflow, wfPath, target string) string {
	dir := filepath.Dir(wfPath)
	if filepath.Base(dir) == ".speakeasy" {
		dir = filepath.Dir(dir)
	}

	if target == "all" {
		if len(wf.Targets) == 1 {
			target = maps.Keys(wf.Targets)[0]
		} else {
			return ""
		}
	}

	outDir := wf.Targets[target].Output
	if outDir != nil {
		if filepath.IsAbs(*outDir) {
			dir = *outDir
		} else {
			dir = filepath.Join(dir, *outDir)
		}
	}

	genConfig, err := sdkGenConfig.Load(dir)
	if err != nil || genConfig.LockFile == nil {
		println("Failed to load gen.lock: %v\n", err)
	}

	lockfileVersion := ""
	if genConfig != nil && genConfig.LockFile != nil {
		lockfileVersion = genConfig.LockFile.Management.SpeakeasyVersion
	}
	return lockfileVersion
}

func updateWorkflowFile(wf *workflow.Workflow, workflowFilepath string) error {
	f, err := os.OpenFile(workflowFilepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("error opening workflow file: %w", err)
	}
	defer f.Close()

	out, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Errorf("error marshalling workflow file: %w", err)
	}

	_, err = f.Write(out)
	if err != nil {
		return fmt.Errorf("error writing to workflow file: %w", err)
	}

	return nil
}
