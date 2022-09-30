package api

import "github.com/spf13/cobra"

func RegisterAPICommands(root *cobra.Command) {
	registerGetApis(root)
	registerGetAllAPIEndpoints(root)
}

func registerGetApis(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-apis",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getApis,
	}
	root.AddCommand(cmd)
}

//nolint:errcheck
func registerGetAllAPIEndpoints(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-all-api-endpoints",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getAllAPIEndpoints,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	root.AddCommand(cmd)
}
