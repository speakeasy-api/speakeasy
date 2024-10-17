package flag

import (
	"encoding/csv"
	"strings"

	"github.com/spf13/cobra"
)

type StringSliceFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 []string
	Deprecated                   bool
	DeprecationMessage           string
}

func (f StringSliceFlag) Init(cmd *cobra.Command) error {
	fullDescription := f.Description + " (comma-separated list)"

	cmd.Flags().StringSliceP(f.Name, f.Shorthand, f.DefaultValue, fullDescription)
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

func (f StringSliceFlag) GetName() string {
	return f.Name
}

func (f StringSliceFlag) ParseValue(v string) (interface{}, error) {
	// Remove the brackets from the string
	v = v[1 : len(v)-1]

	if v == "" {
		return []string{}, nil
	}

	stringReader := strings.NewReader(v)
	csvReader := csv.NewReader(stringReader)
	return csvReader.Read()
}
