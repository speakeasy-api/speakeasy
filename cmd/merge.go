package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge multiple OpenAPI documents into a single document",
	Long: `Merge multiple OpenAPI documents into a single document, useful for merging multiple OpenAPI documents into a single document for generating a client SDK.
Note: That any duplicate operations, components, etc. will be overwritten by the next document in the list.`,
	PreRunE: interactivity.GetMissingFlagsPreRun,
	RunE:    mergeExec,
}

func mergeInit() {
	// TODO: Make the usage description change based on whether its being shown in interactive mode. Use a shared description for all array flags
	mergeCmd.Flags().StringArrayP("schemas", "s", []string{}, "a list of paths to OpenAPI documents to merge, specify -s `path/to/schema1.json` -s `path/to/schema2.json` etc")
	_ = mergeCmd.MarkFlagRequired("schemas")
	mergeCmd.Flags().StringP("out", "o", "", "path to the output file")
	_ = mergeCmd.MarkFlagRequired("out")

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

	if err := merge.MergeOpenAPIDocuments(cmd.Context(), inSchemas, outFile, "", ""); err != nil {
		return err
	}

	log.From(cmd.Context()).Successf("Successfully merged %d schemas into %s", len(inSchemas), outFile)

	return nil
}
