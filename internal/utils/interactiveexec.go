package utils

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
)

func InteractiveExec(cmd *cobra.Command, args []string, label string) error {
	selected := SelectCommand(label, cmd.Commands())

	selected.SetContext(cmd.Context())

	err := GetMissingFlags(selected)
	if err != nil {
		return err
	}

	return selected.RunE(selected, args)
}

func SelectCommand(label string, commands []*cobra.Command) *cobra.Command {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . | cyan | bold }}",
		Active:   "🐝 {{ .Name | yellow | bold }} - {{ .Short | faint }}",
		Inactive: "   {{ .Name | white | bold }} - {{ .Short | faint }}",
		Selected: "> {{ .Name | green | bold }}",
	}

	prompt := promptui.Select{
		HideHelp:  true,
		Label:     label,
		Items:     commands,
		Templates: templates,
		Size:      len(commands),
	}

	index, _, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return commands[index]
}

func InteractiveRunFn(label string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return InteractiveExec(cmd, args, label)
	}
}

func GetMissingFlagsPreRun(cmd *cobra.Command, args []string) error {
	return GetMissingFlags(cmd)
}

func GetMissingFlags(cmd *cobra.Command) error {
	if cmd.Flags().HasAvailableFlags() {
		modifiedFlags, err := RequestFlagValues(cmd.Flags())
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

func RequestFlagValues(flags *pflag.FlagSet) ([]*pflag.Flag, error) {
	values := make([]*pflag.Flag, 0)
	var err error

	requestValue := func(flag *pflag.Flag) {
		// If the flag isn't required, skip it
		if a, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; !ok || a[0] != "true" {
			return
		}

		// If the flag already has a value, skip it
		if flag.Changed {
			return
		}

		templates := &promptui.PromptTemplates{
			Prompt:  "--{{ .Name | yellow | bold }} ({{ .Usage | faint }}): ",
			Valid:   "--{{ .Name | yellow | bold }} ({{ .Usage | faint }}): ",
			Invalid: "--{{ .Name | red | bold }} ({{ .Usage | faint }}): ",
			Success: "--{{ .Name | green | bold }}=",
		}

		prompt := promptui.Prompt{
			Label:     flag,
			Templates: templates,
			Validate:  flag.Value.Set,
		}

		_, runErr := prompt.Run()
		err = runErr

		flag.Changed = true

		values = append(values, flag)
	}

	flags.VisitAll(requestValue)

	return values, err
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
