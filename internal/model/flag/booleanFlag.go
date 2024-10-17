package flag

import (
	"strconv"

	"github.com/spf13/cobra"
)

type BooleanFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 bool
	Deprecated                   bool
	DeprecationMessage           string
}

func (f BooleanFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().BoolP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
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

func (f BooleanFlag) GetName() string {
	return f.Name
}

func (f BooleanFlag) ParseValue(v string) (interface{}, error) {
	return strconv.ParseBool(v)
}
