//nolint:errcheck
package api

import "github.com/spf13/cobra"

func RegisterAPICommands(root *cobra.Command) {
	registerGetApis(root)
	registerGetAllAPIEndpoints(root)
	registerRegisterSchema(root)
	registerGetVersionMetadata(root)
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

func registerRegisterSchema(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "register-schema",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  registerSchema,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("schema", "", "Path to schema to register")
	cmd.MarkFlagRequired("schema")
	root.AddCommand(cmd)
}

func registerGetVersionMetadata(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-version-metadata",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getVersionMetadata,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	root.AddCommand(cmd)
}
