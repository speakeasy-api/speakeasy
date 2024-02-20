package model

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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
	Commands                           []Command
}

func (c CommandGroup) Init() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   c.Usage,
		Short: c.Short,
		Long:  c.Long,
		RunE:  interactivity.InteractiveRunFn(c.InteractiveMsg),
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
	Usage, Short, Long   string
	Flags                []flag.Flag
	PreRun               func(ctx context.Context, flags *F) error
	Run                  func(ctx context.Context, flags F) error
	RunInteractive       func(ctx context.Context, flags F) error
	Hidden, RequiresAuth bool

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
			// check free access
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

// Verify that the command types implement the Command interface
var _ = []Command{
	&ExecutableCommand[interface{}]{},
	&CommandGroup{},
}
