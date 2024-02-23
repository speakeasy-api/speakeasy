package flag

import (
	"github.com/spf13/cobra"
	"strconv"
)

type BooleanFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 bool
}

func (f BooleanFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().BoolP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f BooleanFlag) GetName() string {
	return f.Name
}

func (f BooleanFlag) ParseValue(v string) (interface{}, error) {
	return strconv.ParseBool(v)
}
