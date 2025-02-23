package interactivity

import (
	"fmt"
	"slices"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

	// Don't ask for subcommands if the command itself is runnable
	if isCommandRunnable(cmd) {
		return cmd
	}

	selected := selectCommand(label, subcommands)

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
			running := styles.DimmedItalic.Render("Running command")
			command := styles.Info.Render(utils.GetFullCommandString(cmd))
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

	flags.SortFlags = false

	requestValue := func(flag *pflag.Flag) {
		// If the flag already has a Value, skip it
		if flag.Changed || flag.Hidden || slices.Contains(utils.FlagsToIgnore, flag.Name) {
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
	if !utils.IsInteractive() {
		msg := ""
		for _, flag := range missingRequiredFlags {
			msg += fmt.Sprintf("  --%s (-%s) - %s\n", flag.Name, flag.Shorthand, flag.Usage)
		}

		return nil, fmt.Errorf("missing required flags: \n%s", msg)
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
			// Check if the flag takes an array Value
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
		if ann, ok := flag.Annotations[charm.AutoCompleteAnnotation]; ok && len(ann) > 0 {
			inputs[i].AutocompleteFileExtensions = ann
		}
	}

	multiInputPrompt := NewMultiInput(title, description, required, inputs...)
	return multiInputPrompt.Run()
}

func isCommandRunnable(cmd *cobra.Command) bool {
	onlyHasHelpFlags := true

	if cmd.Flags().HasFlags() {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if !slices.Contains(utils.FlagsToIgnore, flag.Name) {
				onlyHasHelpFlags = false
			}
		})
	}

	return cmd.Runnable() && !onlyHasHelpFlags
}

func isHidden(cmd *cobra.Command) bool {
	if cmd.Hidden {
		return true
	}

	qualifiedName := getFullyQualifiedName(cmd)

	return slices.Contains(CommandsHiddenFromInteractivity, qualifiedName)
}

func getFullyQualifiedName(cmd *cobra.Command) string {
	if cmd.HasParent() && cmd.Parent() != cmd.Root() {
		return getFullyQualifiedName(cmd.Parent()) + "." + cmd.Name()
	}

	return cmd.Name()
}
