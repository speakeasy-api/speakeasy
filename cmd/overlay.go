package cmd

import (
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
	"os"

	"github.com/spf13/cobra"

	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
)

var overlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Work with OpenAPI Overlays",
}

var overlayValidateCmd = &cobra.Command{
	Use:     "validate",
	Short:   "Given an overlay, it will state whether it appears to be valid according to the OpenAPI Overlay specification",
	Args:    cobra.NoArgs,
	PreRunE: utils.GetMissingFlagsPreRun,
	RunE:    RunValidateOverlay,
}

var overlayCompareCmd = &cobra.Command{
	Use:     "compare",
	Short:   "Given two specs, it will output an overlay that describes the differences between them",
	Args:    cobra.NoArgs,
	PreRunE: utils.GetMissingFlagsPreRun,
	RunE:    RunCompare,
}

var overlayApplyCmd = &cobra.Command{
	Use:     "apply",
	Short:   "Given an overlay, it will construct a new specification by extending a specification and applying the overlay, and output it to stdout.",
	Args:    cobra.NoArgs,
	PreRunE: utils.GetMissingFlagsPreRun,
	RunE:    RunApply,
}

func overlayInit() {
	overlayCmd.AddCommand(overlayValidateCmd)
	overlayCmd.AddCommand(overlayCompareCmd)
	overlayCmd.AddCommand(overlayApplyCmd)

	overlayValidateCmd.Flags().StringP("overlay", "o", "", "overlay file to validate")
	overlayValidateCmd.MarkFlagRequired("overlay")

	overlayCompareCmd.Flags().StringSliceP("schemas", "s", []string{}, "schemas to compare and generate overlay from")
	overlayCompareCmd.MarkFlagRequired("schemas")

	overlayApplyCmd.Flags().StringP("overlay", "o", "", "overlay file to apply")
	overlayApplyCmd.MarkFlagRequired("overlay")
	overlayApplyCmd.Flags().StringP("schema", "s", "", "schema to extend (optional)")

	rootCmd.AddCommand(overlayCmd)
}

func RunValidateOverlay(c *cobra.Command, args []string) error {
	overlayFile, err := c.Flags().GetString("overlay")
	if err != nil {
		return err
	}

	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	err = o.Validate()
	if err != nil {
		return err
	}

	fmt.Printf("Overlay file %q is valid.\n", overlayFile)
	return nil
}

func RunCompare(c *cobra.Command, args []string) error {
	schemas, err := c.Flags().GetStringSlice("schemas")
	if err != nil {
		return err
	}

	if len(schemas) != 2 {
		return fmt.Errorf("Exactly two --schemas must be passed to perform a comparison.")
	}

	y1, err := loader.LoadSpecification(schemas[0])
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", schemas[0], err)
	}

	y2, err := loader.LoadSpecification(schemas[1])
	if err != nil {
		return fmt.Errorf("failed to laod %q: %w", schemas[1], err)
	}

	title := fmt.Sprintf("Overlay %s => %s", schemas[0], schemas[1])

	o, err := overlay.Compare(title, schemas[0], y1, *y2)
	if err != nil {
		return fmt.Errorf("failed to compare spec files %q and %q: %w", schemas[0], schemas[1], err)
	}

	err = o.Format(os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to format overlay: %w", err)
	}

	return nil
}

func RunApply(c *cobra.Command, args []string) error {
	overlayFile, err := c.Flags().GetString("overlay")
	if err != nil {
		return err
	}

	schema, err := c.Flags().GetString("schema")
	if err != nil {
		return err
	}

	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	err = o.Validate()
	if err != nil {
		return err
	}

	ys, specFile, err := loader.LoadEitherSpecification(schema, o)
	if err != nil {
		return err
	}

	err = o.ApplyTo(ys)
	if err != nil {
		return fmt.Errorf("failed to apply overlay to spec file %q: %w", specFile, err)
	}

	enc := yaml.NewEncoder(os.Stdout)
	err = enc.Encode(ys)
	if err != nil {
		return fmt.Errorf("failed to encode spec file %q: %w", specFile, err)
	}

	return nil
}
