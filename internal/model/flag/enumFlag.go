package flag

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
)

type EnumFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
	AllowedValues                []string
	Deprecated                   bool
	DeprecationMessage           string
}

func (f EnumFlag) Init(cmd *cobra.Command) error {
	if len(f.AllowedValues) == 0 {
		return fmt.Errorf("allowed values must not be empty")
	}

	if !f.Required {
		if !slices.Contains(f.AllowedValues, f.DefaultValue) {
			return fmt.Errorf("default value %s is not in the list of allowed values", f.DefaultValue)
		}

		if f.DefaultValue == "" {
			return fmt.Errorf("default value must not be empty if the flag is not required")
		}
	}

	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
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

func (f EnumFlag) GetName() string {
	return f.Name
}

func (f EnumFlag) ParseValue(v string) (interface{}, error) {
	return v, nil
}
