//nolint:errcheck
package api

import "github.com/spf13/cobra"

func RegisterAPICommands(root *cobra.Command) {
	cmds := []func(*cobra.Command){
		registerGetApis,
		registerGetAllAPIEndpoints,
		registerGetApiEndpoint,
		registerFindApiEndpoint,
		registerRegisterSchema,
		registerGetVersionMetadata,
		registerGetSchemas,
		registerGetSchemaRevision,
		registerGetSchemaDiff,
		registerDownloadLatestSchema,
		registerDownloadSchemaRevision,
	}

	for _, cmd := range cmds {
		cmd(root)
	}
}

func registerPrintableApiCommand(root *cobra.Command, newCommand *cobra.Command) {
	newCommand.Flags().Bool("json", false, "Output in JSON format")

	root.AddCommand(newCommand)
}

func registerGetApis(root *cobra.Command) {
	registerPrintableApiCommand(root, &cobra.Command{
		Use:   "get-apis",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getApis,
	})
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
	registerPrintableApiCommand(root, cmd)
}

func registerGetApiEndpoint(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-api-endpoint",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getApiEndpoint,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("api-endpoint-id", "", "API Endpoint ID")
	cmd.MarkFlagRequired("api-endpoint-id")
	registerPrintableApiCommand(root, cmd)
}

func registerFindApiEndpoint(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "find-api-endpoint",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  findApiEndpoint,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("display-name", "", "Display name of endpoint")
	cmd.MarkFlagRequired("display-name")
	registerPrintableApiCommand(root, cmd)
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
	registerPrintableApiCommand(root, cmd)
}

func registerGetSchemas(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-schemas",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getSchemas,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	registerPrintableApiCommand(root, cmd)
}

func registerGetSchemaRevision(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-schema-revision",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getSchemaRevision,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("revision-id", "", "Revision ID")
	cmd.MarkFlagRequired("revision-id")
	registerPrintableApiCommand(root, cmd)
}

func registerGetSchemaDiff(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-schema-diff",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  getSchemaDiff,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("base-revision-id", "", "Base revision ID")
	cmd.MarkFlagRequired("base-revision-id")
	cmd.Flags().String("target-revision-id", "", "Target revision ID")
	cmd.MarkFlagRequired("target-revision-id")
	registerPrintableApiCommand(root, cmd)
}

func registerDownloadLatestSchema(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "download-latest-schema",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  downloadLatestSchema,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	root.AddCommand(cmd)
}

func registerDownloadSchemaRevision(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "download-schema-revision",
		Short: "TBD",
		Long:  `TBD`,
		RunE:  downloadSchemaRevision,
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("revision-id", "", "Revision ID")
	cmd.MarkFlagRequired("revision-id")
	root.AddCommand(cmd)
}
