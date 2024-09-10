package flag

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
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
	fmt.Println("Parsing boolean value", f.Name, v)
	return strconv.ParseBool(v)
}
