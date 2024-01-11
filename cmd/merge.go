package cmd

import (
	"fmt"

	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge multiple OpenAPI documents into a single document",
	Long: `Merge multiple OpenAPI documents into a single document, useful for merging multiple OpenAPI documents into a single document for generating a client SDK.
Note: That any duplicate operations, components, etc. will be overwritten by the next document in the list.`,
	PreRunE: utils.GetMissingFlagsPreRun,
	RunE:    mergeExec,
}

func mergeInit() {
	mergeCmd.Flags().StringArrayP("schemas", "s", []string{}, "paths to the openapi schemas to merge")
	mergeCmd.MarkFlagRequired("schemas")
	mergeCmd.Flags().StringP("out", "o", "", "path to the output file")
	mergeCmd.MarkFlagRequired("out")

	rootCmd.AddCommand(mergeCmd)
}

func mergeExec(cmd *cobra.Command, args []string) error {
	inSchemas, err := cmd.Flags().GetStringArray("schemas")
	if err != nil {
		return err
	}

	outFile, err := cmd.Flags().GetString("out")
	if err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(inSchemas, outFile); err != nil {
		return err
	}

	fmt.Println(utils.Green(fmt.Sprintf("Successfully merged %d schemas into %s", len(inSchemas), outFile)))

	return nil
}
