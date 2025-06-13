package flag

import (
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/spf13/cobra"
)

type StringFlagWithOptionalValue struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
	AutocompleteFileExtensions   []string
	Deprecated                   bool
	DeprecationMessage           string
}

func (f StringFlagWithOptionalValue) Init(cmd *cobra.Command) error {
	// Create a regular string flag first
	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)

	// Set the NoOptDefVal to enable optional argument behavior
	flag := cmd.Flags().Lookup(f.Name)
	if flag != nil {
		flag.NoOptDefVal = f.DefaultValue
	}

	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	if f.Deprecated {
		if err := setDeprecated(cmd, f.Name, f.DeprecationMessage); err != nil {
			return err
		}
	}
	if len(f.AutocompleteFileExtensions) > 0 {
		if err := cmd.Flags().SetAnnotation(f.Name, charm.AutoCompleteAnnotation, f.AutocompleteFileExtensions); err != nil {
			return err
		}
	}
	return nil
}

func (f StringFlagWithOptionalValue) GetName() string {
	return f.Name
}

func (f StringFlagWithOptionalValue) ParseValue(v string) (interface{}, error) {
	return v, nil
}
