package interactivity

import (
	"fmt"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
)

func InteractiveExec(cmd *cobra.Command, args []string, label string) error {
	if !utils.IsInteractive() {
		return cmd.Help()
	}

	selected := SelectCommand(label, cmd)
	if selected == nil {
		return nil // Not an error, just exit
	}

	selected.SetContext(cmd.Context())

	err := GetMissingFlags(selected)
	if err != nil {
		return err
	}

	if selected.RunE != nil {
		return selected.RunE(selected, args)
	} else if selected.Run != nil {
		selected.Run(selected, args)
	}

	return nil
}

func SelectCommand(label string, cmd *cobra.Command) *cobra.Command {
	if !cmd.HasSubCommands() {
		return cmd
	}

	rawSubCommands := cmd.Commands()

	if len(rawSubCommands) == 1 && isCommandRunnable(rawSubCommands[0]) && !isHidden(rawSubCommands[0]) {
		return SelectCommand(label, rawSubCommands[0])
	}

	var subcommands []*cobra.Command
	for _, command := range rawSubCommands {
		if !isHidden(command) {
			subcommands = append(subcommands, command)
		}
	}

	// TODO figure out a better way to do this
	if isCommandRunnable(cmd) {
		label = fmt.Sprintf("Continue with %s, or select a subcommand.", cmd.Name())
		subcommands = append([]*cobra.Command{cmd}, subcommands...)
	}

	selected := getSelectionFromList(label, subcommands)

	if selected != nil && selected != cmd && selected.HasSubCommands() {
		return SelectCommand(label, selected)
	}

	return selected
}

type RunE = func(cmd *cobra.Command, args []string) error

func InteractiveRunFn(label string) RunE {
	return func(cmd *cobra.Command, args []string) error {
		return InteractiveExec(cmd, args, label)
	}
}

func GetMissingFlagsPreRun(cmd *cobra.Command, args []string) error {
	return GetMissingFlags(cmd)
}

func GetMissingFlags(cmd *cobra.Command) error {
	if cmd.Flags().HasAvailableFlags() {
		modifiedFlags, err := RequestFlagValues(cmd.CommandPath(), cmd.Flags())
		if err != nil {
			return err
		}

		// Only print if there were flags missing. This avoids printing when the full command was provided initially.
		if len(modifiedFlags) > 0 {
			flagString := ""
			for _, flag := range getSetFlags(cmd.Flags()) {
				flagString += fmt.Sprintf(" --%s=%s", flag.Name, flag.Value)
			}

			running := styles.DimmedItalic.Render("Running command")
			command := styles.Info.Render(fmt.Sprintf(`%s%s`, cmd.CommandPath(), flagString))
			log.From(cmd.Context()).Printf("\n%s %s\n", running, command)
		}
	}

	return nil
}

// RequestFlagValues returns the flags that were modified
func RequestFlagValues(commandName string, flags *pflag.FlagSet) ([]*pflag.Flag, error) {
	values := make([]*pflag.Flag, 0)
	var err error

	var missingRequiredFlags []*pflag.Flag
	var missingOptionalFlags []*pflag.Flag

	flagsToIgnore := []string{"help", "version", "logLevel"}

	requestValue := func(flag *pflag.Flag) {
		// If the flag already has a value, skip it
		if flag.Changed || flag.Hidden || slices.Contains(flagsToIgnore, flag.Name) {
			return
		}

		// Checks flag optionality
		if a, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; !ok || a[0] != "true" {
			missingOptionalFlags = append(missingOptionalFlags, flag)
		} else {
			missingRequiredFlags = append(missingRequiredFlags, flag)
		}
	}

	flags.VisitAll(requestValue)

	// If all required flags were already provided, don't ask for any more values
	if len(missingRequiredFlags) == 0 {
		return nil, nil
	}

	requiredValues := requestFlagValues(commandName, true, missingRequiredFlags)
	if len(requiredValues) != len(missingRequiredFlags) {
		return nil, nil // User exited
	}

	optionalValues := requestFlagValues(commandName, false, missingOptionalFlags)

	setValue := func(flag *pflag.Flag) {
		var v string
		if _, ok := requiredValues[flag.Name]; ok {
			v = requiredValues[flag.Name]
		} else if _, ok := optionalValues[flag.Name]; ok {
			v = optionalValues[flag.Name]
		}

		if v != "" {
			// Check if the flag takes an array value
			if sliceVal, ok := flag.Value.(pflag.SliceValue); ok {
				vals := strings.Split(v, ",")
				if err := sliceVal.Replace(vals); err != nil {
					return
				}
			} else {
				if err := flag.Value.Set(v); err != nil {
					return
				}
			}
			flag.Changed = true
		}
	}

	flags.VisitAll(setValue)

	values = append(values, missingRequiredFlags...)
	values = append(values, missingOptionalFlags...)

	return values, err
}

func requestFlagValues(title string, required bool, flags []*pflag.Flag) map[string]string {
	if len(flags) == 0 {
		return nil
	}

	description := "Optionally supply values for the following"
	if required {
		description = "Please supply values for the required fields"
	}

	inputs := make([]InputField, len(flags))
	for i, flag := range flags {
		inputs[i] = InputField{Name: flag.Name, Placeholder: flag.Usage}
	}

	multiInputPrompt := NewMultiInput(title, description, required, inputs...)
	return multiInputPrompt.Run()
}

func getSetFlags(flags *pflag.FlagSet) []*pflag.Flag {
	values := make([]*pflag.Flag, 0)

	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			values = append(values, flag)
		}
	})

	return values
}

func isCommandRunnable(cmd *cobra.Command) bool {
	onlyHasHelpFlags := cmd.Flags().HasFlags()

	if cmd.Flags().HasFlags() {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if flag.Name != "help" && flag.Name != "version" && flag.Name != "logLevel" {
				onlyHasHelpFlags = false
			}
		})
	}

	return cmd.Runnable() && !onlyHasHelpFlags
}

func isHidden(cmd *cobra.Command) bool {
	_, hasHiddenAnnotation := cmd.Annotations["hide"]
	return cmd.Hidden || hasHiddenAnnotation || cmd.Name() == "completion" || cmd.Name() == "help"
}
