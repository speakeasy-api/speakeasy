package flag

import (
	"github.com/spf13/cobra"
)

type Flag interface {
	Init(cmd *cobra.Command) error
	GetName() string
	ParseValue(v string) (interface{}, error)
}

func setRequiredAndHidden(cmd *cobra.Command, name string, required, hidden bool) error {
	if required {
		if err := cmd.MarkFlagRequired(name); err != nil {
			return err
		}
	}
	if hidden {
		if err := cmd.Flags().MarkHidden(name); err != nil {
			return err
		}
	}

	return nil
}

func setDeprecated(cmd *cobra.Command, name, message string) error {
	return cmd.Flags().MarkDeprecated(name, message)
}

// Verify that the flag types implement the Flag interface
var _ = []Flag{
	&StringFlag{},
	&BooleanFlag{},
	&IntFlag{},
	&MapFlag{},
	&StringSliceFlag{},
}
