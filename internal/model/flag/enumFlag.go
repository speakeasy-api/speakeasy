package flag

import (
	"fmt"
	"github.com/spf13/cobra"
	"slices"
	"strings"
)

type EnumFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
	AllowedValues                []string
}

func (f EnumFlag) Init(cmd *cobra.Command) error {
	if len(f.AllowedValues) == 0 {
		return fmt.Errorf("allowed values must not be empty")
	}

	defaultString := ""
	if !f.Required {
		if !slices.Contains(f.AllowedValues, f.DefaultValue) {
			return fmt.Errorf("default value %s is not in the list of allowed values", f.DefaultValue)
		}

		if f.DefaultValue == "" {
			return fmt.Errorf("default value must not be empty if the flag is not required")
		}
		defaultString = fmt.Sprintf(", default: %s", f.DefaultValue)
	}
	fullDescription := fmt.Sprintf("%s (one of: %s%s)", f.Description, strings.Join(f.AllowedValues, ", "), defaultString)

	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, fullDescription)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}

	return nil
}

func (f EnumFlag) GetName() string {
	return f.Name
}

func (f EnumFlag) ParseValue(v string) (interface{}, error) {
	return v, nil
}
