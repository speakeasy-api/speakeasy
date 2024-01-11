package utils

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
	"os"
)

func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func InteractiveExec(cmd *cobra.Command, args []string, label string) error {
	if !IsInteractive() {
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

	subcommands := cmd.Commands()

	if len(subcommands) == 1 && isCommandRunnable(subcommands[0]) {
		return SelectCommand(label, subcommands[0])
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

			runningString := promptui.Styler(promptui.FGFaint, promptui.FGItalic)("Running command:")
			commandString := promptui.Styler(promptui.FGCyan, promptui.FGBold)(fmt.Sprintf(`%s%s`, cmd.CommandPath(), flagString))
			println(fmt.Sprintf("\n%s %s\n", runningString, commandString))
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

	requestValue := func(flag *pflag.Flag) {
		// If the flag already has a value, skip it
		if flag.Changed || flag.Hidden || flag.Name == "help" || flag.Name == "version" {
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
			if err := flag.Value.Set(v); err != nil {
				return
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
			if flag.Name != "help" && flag.Name != "version" {
				onlyHasHelpFlags = false
			}
		})
	}

	return cmd.Runnable() && !onlyHasHelpFlags
}
