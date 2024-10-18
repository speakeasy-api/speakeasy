package flag

import (
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/spf13/cobra"
)

type StringFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
	AutocompleteFileExtensions   []string
	Deprecated                   bool
	DeprecationMessage           string
}

func (f StringFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
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

func (f StringFlag) GetName() string {
	return f.Name
}

func (f StringFlag) ParseValue(v string) (interface{}, error) {
	return v, nil
}
