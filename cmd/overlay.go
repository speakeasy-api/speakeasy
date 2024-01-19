package cmd

import (
	"os"

	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"

	"github.com/spf13/cobra"

	"github.com/speakeasy-api/speakeasy/internal/overlay"
)

var overlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Work with OpenAPI Overlays",
}

var overlayValidateCmd = &cobra.Command{
	Use:     "validate",
	Short:   "Given an overlay, it will state whether it appears to be valid according to the OpenAPI Overlay specification",
	Args:    cobra.NoArgs,
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    runValidateOverlay,
}

var overlayCompareCmd = &cobra.Command{
	Use:     "compare",
	Short:   "Given two specs, it will output an overlay that describes the differences between them",
	Args:    cobra.NoArgs,
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    runCompare,
}

var overlayApplyCmd = &cobra.Command{
	Use:     "apply",
	Short:   "Given an overlay, it will construct a new specification by extending a specification and applying the overlay, and output it to stdout.",
	Args:    cobra.NoArgs,
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    runApply,
}

func overlayInit() {
	overlayCmd.AddCommand(overlayApplyCmd)
	overlayCmd.AddCommand(overlayValidateCmd)
	overlayCmd.AddCommand(overlayCompareCmd)

	overlayValidateCmd.Flags().StringP("overlay", "o", "", "overlay file to validate")
	overlayValidateCmd.MarkFlagRequired("overlay")

	overlayCompareCmd.Flags().StringSliceP("schemas", "s", []string{}, "schemas to compare and generate overlay from")
	overlayCompareCmd.MarkFlagRequired("schemas")

	overlayApplyCmd.Flags().StringP("overlay", "o", "", "overlay file to apply")
	overlayApplyCmd.MarkFlagRequired("overlay")
	overlayApplyCmd.Flags().StringP("schema", "s", "", "schema to extend (optional)")

	rootCmd.AddCommand(overlayCmd)
}

func runValidateOverlay(c *cobra.Command, args []string) error {
	overlayFile, err := c.Flags().GetString("overlay")
	if err != nil {
		return err
	}

	if err := overlay.Validate(overlayFile); err != nil {
		return err
	}

	log.From(c.Context()).Successf("Overlay file %q is valid.", overlayFile)
	return nil
}

func runCompare(c *cobra.Command, args []string) error {
	schemas, err := c.Flags().GetStringSlice("schemas")
	if err != nil {
		return err
	}

	return overlay.Compare(schemas)
}

func runApply(c *cobra.Command, args []string) error {
	overlayFile, err := c.Flags().GetString("overlay")
	if err != nil {
		return err
	}

	schema, err := c.Flags().GetString("schema")
	if err != nil {
		return err
	}

	return overlay.Apply(schema, overlayFile, os.Stdout)
}
