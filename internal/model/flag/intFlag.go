package flag

import (
	"strconv"

	"github.com/spf13/cobra"
)

type IntFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 int
	Deprecated                   bool
	DeprecationMessage           string
}

func (f IntFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().IntP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	if f.Deprecated {
		if err := setDeprecated(cmd, f.Name, f.DeprecationMessage); err != nil {
			return err
		}
	}
	return nil
}

func (f IntFlag) GetName() string {
	return f.Name
}

func (f IntFlag) ParseValue(v string) (interface{}, error) {
	return strconv.Atoi(v)
}
