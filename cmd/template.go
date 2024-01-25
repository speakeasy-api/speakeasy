package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/template"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "executes your template to render an OpenAPI file.",
	Args:  cobra.NoArgs,
	RunE:  runTemplate,
}

func templateInit() {
	templateCmd.Flags().StringP("template", "t", "", "template file")
	templateCmd.MarkFlagRequired("template")
	templateCmd.Flags().StringP("values", "v", "", "values file")
	templateCmd.MarkFlagRequired("values")
	templateCmd.Flags().StringP("out", "o", "", "output location")
	templateCmd.MarkFlagRequired("out")

	rootCmd.AddCommand(templateCmd)
}

func runTemplate(cmd *cobra.Command, args []string) error {
	templateLocation, err := cmd.Flags().GetString("template")
	if err != nil {
		return err
	}

	valuesLocation, err := cmd.Flags().GetString("values")
	if err != nil {
		return err
	}

	outLocation, err := cmd.Flags().GetString("out")
	if err != nil {
		return err
	}

	return template.Execute(templateLocation, valuesLocation, outLocation)
}
