package flag

import "github.com/spf13/cobra"

type StringFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
}

func (f StringFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f StringFlag) GetName() string {
	return f.Name
}

func (f StringFlag) ParseValue(v string) (interface{}, error) {
	return v, nil
}
