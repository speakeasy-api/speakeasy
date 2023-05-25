package utils

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func InteractiveExec(cmd *cobra.Command, args []string, label string) error {
	if !IsInteractive() {
		return cmd.Help()
	}

	selected := SelectCommand(label, cmd)

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

	commands := cmd.Commands()

	if len(commands) == 1 && isCommandRunnable(commands[0]) {
		return SelectCommand(label, commands[0])
	}

	if isCommandRunnable(cmd) {
		commands = append([]*cobra.Command{cmd}, commands...)
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{.}}",
		Active:   "🐝 {{ .Name | yellow | bold }} - {{ .Short | faint }}",
		Inactive: "   {{ .Name | white | bold }} - {{ .Short | faint }}",
		Selected: "> {{ .Name | green | bold }}",
	}

	prompt := promptui.Select{
		HideHelp:  true,
		Label:     "",
		Items:     commands,
		Templates: templates,
		Size:      len(commands),
	}

	fmt.Println(promptui.Styler(promptui.FGCyan, promptui.FGBold)(label))
	index, _, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	selected := commands[index]

	if selected != cmd && selected.HasSubCommands() {
		return SelectCommand(label, selected)
	}

	return selected
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
