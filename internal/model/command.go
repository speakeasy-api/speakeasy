package model

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/run"

	"github.com/speakeasy-api/speakeasy-core/errors"

	"github.com/fatih/structs"
	"github.com/hashicorp/go-version"
	"github.com/sethvargo/go-githubactions"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const ErrInstallFailed = errors.Error("failed to install Speakeasy version")

type Command interface {
	Init() (*cobra.Command, error) // TODO: make private when rootCmd is refactored?
}

type CommandGroup struct {
	Usage, Short, Long, InteractiveMsg string
	Aliases                            []string
	Commands                           []Command
	Hidden                             bool
}

func (c CommandGroup) Init() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     c.Usage,
		Short:   c.Short,
		Long:    c.Long,
		Aliases: c.Aliases,
		RunE:    interactivity.InteractiveRunFn(c.InteractiveMsg),
		Hidden:  c.Hidden,
	}

	for _, subcommand := range c.Commands {
		subcmd, err := subcommand.Init()
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(subcmd)
	}

	return cmd, nil
}

// ExecutableCommand is a runnable "leaf" command that can be executed directly and has no subcommands
// F is a struct type that represents the flags for the command. The json tags on the struct fields are used to map to the command line flags
type ExecutableCommand[F interface{}] struct {
	Usage, Short, Long string
	Aliases            []string
	Flags              []flag.Flag
	PreRun             func(cmd *cobra.Command, flags *F) error
	Run                func(ctx context.Context, flags F) error
	RunInteractive     func(ctx context.Context, flags F) error

	// When enabled, this command is not included in the list of available
	// commands.
	Hidden bool

	// When enabled, the command requires authentication. If not authenticated
	// and the execution environment is local, the user will be prompted to
	// authenticate. If not authenticated and the execution environment is not
	// local, an error will be returned. Authentication information, such as
	// workspace identifier and account type, will be added to the command
	// context.
	RequiresAuth bool

	// When enabled, the command uses a workflow file. If the "pinned" CLI flag
	// is not present or set to false and the execution environment is not
	// local, run using the CLI version specified in the workflow file.
	UsesWorkflowFile bool

	// Deprecated: try to avoid using this. It is only present for backwards compatibility with the old CLI
	NonInteractiveSubcommands []Command
}

func (c ExecutableCommand[F]) Init() (*cobra.Command, error) {
	run := func(cmd *cobra.Command, args []string) error {
		// TODO: This is currently redundant, but seems to have some effect on later flag parsing...
		if err := interactivity.GetMissingFlagsPreRun(cmd, args); err != nil {
			return err
		}

		flags, err := c.GetFlagValues(cmd)
		if err != nil {
			return err
		}

		if c.PreRun != nil {
			if err := c.PreRun(cmd, flags); err != nil {
				return err
			}
		}

		if c.RequiresAuth {
			authCtx, err := auth.Authenticate(cmd.Context(), false)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			cmd.SetContext(authCtx)
		} else {
			authCtx, err := auth.UseExistingAPIKeyIfAvailable(cmd.Context())
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			cmd.SetContext(authCtx)
		}

		// If the command uses a workflow file, run using the version specified in the workflow file
		if c.UsesWorkflowFile {
			// If we're running locally or the --pinned flag is set simply run the command normally with the existing version of the CLI
			pinned, _ := cmd.Flags().GetBool("pinned")
			if !pinned && !env.IsLocalDev() {
				err := runWithVersionFromWorkflowFile(cmd)
				if err == nil {
					return nil
				} else if !errors.Is(err, ErrInstallFailed) { // Don't fail on download failure. Proceed using the current CLI version, as if it was run with --pinned
					return err
				}
				logger := log.From(cmd.Context())
				logger.PrintfStyled(styles.DimmedItalic, "Failed to download latest Speakeasy version: %s", err.Error())
				logger.PrintfStyled(styles.DimmedItalic, "Running with local version. This might result in inconsistencies between environments\n")
			}
		}

		mustRunInteractive := c.RunInteractive != nil && utils.IsInteractive() && !env.IsGithubAction()

		if !mustRunInteractive && c.Run == nil {
			return fmt.Errorf("this command is only available in an interactive terminal")
		}

		execute := func(ctx context.Context) error {
			if mustRunInteractive {
				return c.RunInteractive(ctx, *flags)
			} else {
				return c.Run(ctx, *flags)
			}
		}

		return events.Telemetry(cmd.Context(), shared.InteractionTypeCliExec, func(ctx context.Context, event *shared.CliEvent) error {
			return execute(ctx)
		})
	}

	// Assert that the flags are valid
	if err := c.checkFlags(); err != nil {
		return nil, err
	}

	short := strings.Trim(c.Short, " .")
	short = utils.CapitalizeFirst(short)

	cmd := &cobra.Command{
		Use:     c.Usage,
		Short:   c.Short,
		Long:    c.Long,
		Aliases: c.Aliases,
		PreRunE: interactivity.GetMissingFlagsPreRun,
		RunE:    run,
		Hidden:  c.Hidden,
	}

	for _, subcommand := range c.NonInteractiveSubcommands {
		subcmd, err := subcommand.Init()
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(subcmd)
	}

	for _, flag := range c.Flags {
		if err := flag.Init(cmd); err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

func (c ExecutableCommand[F]) checkFlags() error {
	var f F
	fields := structs.Fields(f)

	tags := make([]string, len(fields))
	for i, field := range fields {
		tags[i] = field.Tag("json")
	}

	for _, flag := range c.Flags {
		if !slices.Contains(tags, flag.GetName()) {
			return fmt.Errorf("flag %s is missing from flags type for command %s", flag.GetName(), c.Usage)
		}
	}

	return nil
}

func (c ExecutableCommand[F]) GetFlagValues(cmd *cobra.Command) (*F, error) {
	var flagValues F

	findFlagDef := func(name string) flag.Flag {
		if slices.Contains(utils.FlagsToIgnore, name) {
			return nil
		}
		for _, f := range c.Flags {
			if f.GetName() == name {
				return f
			}
		}
		return nil
	}

	jsonFlags := make(map[string]interface{})
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		flag := findFlagDef(f.Name)
		if flag == nil {
			return
		}

		v, err := flag.ParseValue(f.Value.String())
		if err != nil {
			panic(err)
		}
		jsonFlags[f.Name] = v
	})

	jsonBytes, err := json.Marshal(jsonFlags)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonBytes, &flagValues); err != nil {
		return nil, err
	}

	return &flagValues, nil
}

// If the command is run from a workflow file, check if the desired version is different from the current version
// If so, download the desired version and run the command with it as a subprocess
// CAUTION: THIS CODE RUNS FOR EVERY EXECUTION OF `run` REGARDLESS OF VERSION PINNING. CHANGES HERE CAN
// BREAK EVEN SDKs THAT ARE PINNED TO A SPECIFIC VERSION.
func runWithVersionFromWorkflowFile(cmd *cobra.Command) error {
	ctx := cmd.Context()
	logger := log.From(ctx)

	wf, wfPath, err := utils.GetWorkflow()
	if err != nil {
		return fmt.Errorf("failed to load workflow file: %w", err)
	}

	artifactArch := ctx.Value(updates.ArtifactArchContextKey).(string)

	// Try to migrate existing workflows, but only if they aren't on a pinned version
	if wf.SpeakeasyVersion.String() == "latest" {
		run.Migrate(ctx, wf)
		_ = updateWorkflowFile(wf, wfPath)
	}

	// Get the latest version, or use the pinned version
	desiredVersion := wf.SpeakeasyVersion.String()
	if desiredVersion == "latest" {
		latest, err := updates.GetLatestVersion(ctx, artifactArch)
		if err != nil {
			return ErrInstallFailed
		}
		desiredVersion = latest.String()

		logger.PrintfStyled(styles.DimmedItalic, "Running with latest Speakeasy version\n")
	} else {
		logger.PrintfStyled(styles.DimmedItalic, "Running with speakeasyVersion defined in workflow.yaml\n")
	}

	// Get lockfile version before running the command, in case it gets overwritten
	lockfileVersion := getSpeakeasyVersionFromLockfile()

	// If the workflow succeeds on latest, promote that version to the default
	shouldPromote := wf.SpeakeasyVersion == "latest"

	runErr := runWithVersion(cmd, artifactArch, desiredVersion, shouldPromote)
	if runErr != nil {
		// If the command failed to run with the latest version, try to run with the version from the lock file
		if wf.SpeakeasyVersion == "latest" {
			msg := fmt.Sprintf("Failed to run with Speakeasy version %s: %s\n", desiredVersion, runErr.Error())
			_ = log.SendToLogProxy(ctx, log.LogProxyLevelError, msg, nil)
			logger.PrintfStyled(styles.DimmedItalic, msg)
			if env.IsGithubAction() {
				githubactions.AddStepSummary("# Speakeasy Version upgrade failure\n" + msg)
			}

			if lockfileVersion != "" && lockfileVersion != desiredVersion {
				logger.PrintfStyled(styles.DimmedItalic, "Rerunning with previous successful version")
				return runWithVersion(cmd, artifactArch, lockfileVersion, false)
			}
		}

		// If the command failed to run with the pinned version, fail normally
		return runErr
	}

	return nil
}

// If promote is true, the version will be promoted to the default version (ie when running `speakeasy`)
func runWithVersion(cmd *cobra.Command, artifactArch, desiredVersion string, shouldPromote bool) error {
	vLocation, err := updates.InstallVersion(cmd.Context(), desiredVersion, artifactArch, 30)
	if err != nil {
		return ErrInstallFailed.Wrap(err)
	}

	cmdParts := utils.GetCommandParts(cmd)
	if cmdParts[0] == "speakeasy" {
		cmdParts = cmdParts[1:]
	}

	// The pinned flag was introduced in 1.256.0
	// For earlier versions, it isn't necessary because we don't try auto-upgrading
	if ok, _ := pinningWasReleased(desiredVersion); ok {
		cmdParts = append(cmdParts, "--pinned")
	}

	newCmd := exec.Command(vLocation, cmdParts...)
	newCmd.Stdin = os.Stdin
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	if err = newCmd.Run(); err != nil {
		return fmt.Errorf("failed to run with version %s: %w", desiredVersion, err)
	}

	// If the workflow succeeded, make the used version the default
	if shouldPromote && !env.IsGithubAction() && !env.IsLocalDev() {
		currentExecPath, err := os.Executable()
		if err != nil {
			log.From(cmd.Context()).Warnf("failed to promote version: %s", err.Error())
			return nil
		}

		if err := os.Rename(vLocation, currentExecPath); err != nil {
			log.From(cmd.Context()).Warnf("failed to promote version: %s", err.Error())
			return nil
		}
	}

	return nil
}

func pinningWasReleased(v string) (bool, error) {
	desiredV, err := version.NewVersion(v)
	if err != nil {
		return false, err
	}

	minVersionForPinnedFlag, err := version.NewVersion("1.256.0")
	if err != nil {
		return false, err
	}

	return desiredV.GreaterThan(minVersionForPinnedFlag), nil
}

func getSpeakeasyVersionFromLockfile() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	workflowLock, err := workflow.LoadLockfile(wd)
	if err != nil || workflowLock == nil {
		return ""
	}

	return workflowLock.SpeakeasyVersion
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

// Verify that the command types implement the Command interface
var _ = []Command{
	&ExecutableCommand[interface{}]{},
	&CommandGroup{},
}
