package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/updates"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/fatih/structs"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Command interface {
	Init() (*cobra.Command, error) // TODO: make private when rootCmd is refactored?
}

type CommandGroup struct {
	Usage, Short, Long, InteractiveMsg string
	Aliases                            []string
	Commands                           []Command
}

func (c CommandGroup) Init() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     c.Usage,
		Short:   c.Short,
		Long:    c.Long,
		Aliases: c.Aliases,
		RunE:    interactivity.InteractiveRunFn(c.InteractiveMsg),
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
	Usage, Short, Long                     string
	Flags                                  []flag.Flag
	PreRun                                 func(ctx context.Context, flags *F) error
	Run                                    func(ctx context.Context, flags F) error
	RunInteractive                         func(ctx context.Context, flags F) error
	Hidden, RequiresAuth, UsesWorkflowFile bool

	// Deprecated: try to avoid using this. It is only present for backwards compatibility with the old CLI
	NonInteractiveSubcommands []Command
}

func (c ExecutableCommand[F]) Init() (*cobra.Command, error) {
	run := func(cmd *cobra.Command, args []string) error {
		if c.RequiresAuth {
			authCtx, err := auth.Authenticate(cmd.Context(), false)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}
			cmd.SetContext(authCtx)
		}

		flags, err := c.GetFlagValues(cmd)
		if err != nil {
			return err
		}

		if c.PreRun != nil {
			if err := c.PreRun(cmd.Context(), flags); err != nil {
				return err
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

	cmd := &cobra.Command{
		Use:     c.Usage,
		Short:   c.Short,
		Long:    c.Long,
		PreRunE: interactivity.GetMissingFlagsPreRun,
		RunE:    run,
		Hidden:  c.Hidden,
	}

	if c.UsesWorkflowFile {
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			if err := runWithVersionFromWorkflowFile(cmd, args); err != nil {
				return err
			}

			return interactivity.GetMissingFlagsPreRun(cmd, args)
		}
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
func runWithVersionFromWorkflowFile(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	logger := log.From(ctx)

	wf, wfPath, err := utils.GetWorkflow()
	if err != nil {
		return fmt.Errorf("failed to load workflow file: %w", err)
	}

	if wf == nil {
		return nil
	}

	currentVersion := events.GetSpeakeasyVersionFromContext(ctx)

	// Try to migrate existing workflows
	if wf.SpeakeasyVersion == "" {
		if ghPinned := env.PinnedVersion(); ghPinned != "" {
			wf.SpeakeasyVersion = workflow.Version(ghPinned)
			wf.VersionLocked = true
		} else {
			wf.SpeakeasyVersion = workflow.Version(currentVersion)
		}

		_ = updateWorkflowFile(wf, wfPath)
	}

	if wf.SpeakeasyVersion != "" && wf.SpeakeasyVersion != "latest" {
		artifactArch := ctx.Value(updates.ArtifactArchContextKey).(string)
		desiredVersion := wf.SpeakeasyVersion.String()

		// If the desired version is the same as the currently running version of the CLI, just run the command
		// Caveat: this means we might not auto-upgrade in certain cases
		if desiredVersion == currentVersion {
			return nil
		}

		newerVersion, err := updates.GetNewerVersion(artifactArch, desiredVersion)
		if err != nil {
			return err
		}

		if wf.VersionLocked || newerVersion == nil {
			if err := runWithVersion(cmd, artifactArch, desiredVersion); err != nil {
				return err
			}
		} else {
			logger.PrintfStyled(styles.DimmedItalic, "Newer Speakeasy version found (%s => %s). Attempting auto-upgrade\n", desiredVersion, newerVersion.String())

			err = runWithVersion(cmd, artifactArch, newerVersion.String())
			if err != nil {
				msg := fmt.Sprintf("Failed to auto-upgrade to version %s: %s\n", newerVersion.String(), err.Error())
				if err = log.SendToLogProxy(ctx, log.LogProxyLevelError, msg, nil); err != nil {
					logger.Errorf("Failed to send log to LogProxy: %v\n", err)
				}

				logger.PrintfStyled(styles.DimmedItalic, msg)
				logger.PrintfStyled(styles.DimmedItalic, "Rerunning with version defined in workflow.yaml: %s\n", desiredVersion)

				if err := runWithVersion(cmd, artifactArch, desiredVersion); err != nil {
					return err
				}
			} else {
				wf.SpeakeasyVersion = workflow.Version(newerVersion.String())
				if err := updateWorkflowFile(wf, wfPath); err != nil {
					return err
				}
				logger.Successf("\nAuto-upgrade successful! Updated workflow.yaml/speakeasyVersion to %s\n", newerVersion.String())
			}
		}

		// Exit here to prevent the command from running twice
		os.Exit(0)
	}

	return nil
}

func runWithVersion(cmd *cobra.Command, artifactArch, desiredVersion string) error {
	ctx := cmd.Context()

	vLocation, err := updates.InstallVersion(ctx, desiredVersion, artifactArch, 30)
	if err != nil {
		return err
	}

	cmdString := utils.GetFullCommandString(cmd)
	cmdString = strings.TrimPrefix(cmdString, "speakeasy ")

	newCmd := exec.Command(vLocation, cmdString)
	newCmd.Stdin = os.Stdin
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr

	err = newCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run with version %s: %w", desiredVersion, err)
	}

	return nil
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
