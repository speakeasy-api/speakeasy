package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/structs"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"slices"
	"strconv"
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
	Flags                []Flag
	PreRun               func(ctx context.Context, flags *F) error
	Run                  func(ctx context.Context, flags F) error
	RunInteractive       func(ctx context.Context, flags F) error
	Hidden, RequiresAuth bool

	// Deprecated: try to avoid using this
	NonInteractiveSubcommands []Command
}

func (c ExecutableCommand[F]) Init() (*cobra.Command, error) {
	run := func(cmd *cobra.Command, args []string) error {
		if c.RequiresAuth {
			if err := auth.Authenticate(cmd.Context(), false); err != nil {
				return err
			}
		}

		flags, err := c.GetFlags(cmd)
		if err != nil {
			return err
		}

		if c.PreRun != nil {
			if err := c.PreRun(cmd.Context(), flags); err != nil {
				return err
			}
		}

		if c.RunInteractive != nil && utils.IsInteractive() && !env.IsGithubAction() {
			return c.RunInteractive(cmd.Context(), *flags)
		} else if c.Run != nil {
			return c.Run(cmd.Context(), *flags)
		} else {
			return fmt.Errorf("this command is only available in an interactive terminal")
		}
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
		if err := flag.init(cmd); err != nil {
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
		if !slices.Contains(tags, flag.getName()) {
			return fmt.Errorf("flag %s is missing from flags type for command %s", flag.getName(), c.Usage)
		}
	}

	return nil
}

func (c ExecutableCommand[F]) GetFlags(cmd *cobra.Command) (*F, error) {
	var flags F

	jsonFlags := make(map[string]interface{})
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() == "string" {
			jsonFlags[f.Name] = f.Value.String()
		} else if f.Value.Type() == "bool" {
			b, err := strconv.ParseBool(f.Value.String())
			if err != nil {
				return
			}
			jsonFlags[f.Name] = b
		} else if f.Value.Type() == "int" {
			i, err := strconv.Atoi(f.Value.String())
			if err != nil {
				return
			}
			jsonFlags[f.Name] = i
		}
	})

	jsonBytes, err := json.Marshal(jsonFlags)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonBytes, &flags); err != nil {
		return nil, err
	}

	return &flags, nil
}

// Verify that the command types implement the Command interface
var _ = []Command{
	&ExecutableCommand[interface{}]{},
	&CommandGroup{},
}
