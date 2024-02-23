//nolint:errcheck
package api

import (
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/spf13/cobra"
)

func RegisterAPICommands(root *cobra.Command) {
	cmds := []func(*cobra.Command){
		registerGetApis,
		registerGetApiVersions,
		registerGenerateOpenAPISpec,
		registerGeneratePostmanCollection,
		registerGetAllAPIEndpoints,
		registerGetAllAPIEndpointsForVersion,
		registerGetApiEndpoint,
		registerFindApiEndpoint,
		registerGenerateOpenAPISpecForAPIEndpoint,
		registerGeneratePostmanCollectionForAPIEndpoint,
		registerRegisterSchema,
		registerGetSchemas,
		registerGetSchemaRevision,
		registerGetSchemaDiff,
		registerDownloadLatestSchema,
		registerDownloadSchemaRevision,
		registerGetVersionMetadata,
		registerQueryEventLog,
		registerGetRequestFromEventLog,
		registerGetValidEmbedAccessTokens,
	}

	for _, cmd := range cmds {
		cmd(root)
	}
}

func registerPrintableApiCommand(root *cobra.Command, newCommand *cobra.Command) {
	newCommand.Flags().Bool("json", false, "Output in JSON format")

	root.AddCommand(newCommand)
}

func authCommand(exec func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		authCtx, err := auth.Authenticate(cmd.Context(), false, "")
		if err != nil {
			return err
		}
		cmd.SetContext(authCtx)

		return exec(cmd, args)
	}
}

func registerGetApis(root *cobra.Command) {
	registerPrintableApiCommand(root, &cobra.Command{
		Use:   "get-apis",
		Short: "Get all Apis",
		Long:  `Get a list of all Apis and there versions for a given workspace`,
		RunE:  authCommand(getApis),
	})
}

func registerGetApiVersions(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-api-versions",
		Short: "Get Api versions",
		Long:  `Get all Api versions for a particular apiID`,
		RunE:  authCommand(getApiVersions),
	}

	cmd.Flags().String("api-id", "", "Api ID")
	cmd.MarkFlagRequired("api-id")

	registerPrintableApiCommand(root, cmd)
}

func registerGenerateOpenAPISpec(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "generate-openapi-spec",
		Short: "Generate OpenAPI spec",
		Long:  `Generate an OpenAPI specification for a particular Api`,
		RunE:  authCommand(generateOpenAPISpec),
	}
	cmd.Flags().String("api-id", "", "Api ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().Bool("diff", false, "Show diff to current version of schema (if available)")
	root.AddCommand(cmd)
}

func registerGeneratePostmanCollection(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "generate-postman-collection",
		Short: "Generate Postman collection",
		Long:  `Generate a Postman collection for a particular Api`,
		RunE:  authCommand(generatePostmanCollection),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	root.AddCommand(cmd)
}

func registerGetAllAPIEndpoints(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-all-api-endpoints",
		Short: "Get all API endpoints",
		Long:  `Get all Api endpoints for a particular apiID`,
		RunE:  authCommand(getAllAPIEndpoints),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	registerPrintableApiCommand(root, cmd)
}

func registerGetAllAPIEndpointsForVersion(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-all-api-endpoints-for-version",
		Short: "Get all API endpoints for version",
		Long:  `Get all ApiEndpoints for a particular apiID and versionID`,
		RunE:  authCommand(getAllAPIEndpointsForVersion),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	registerPrintableApiCommand(root, cmd)
}

func registerGetApiEndpoint(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-api-endpoint",
		Short: "Get ApiEndpoint",
		Long:  `Get an ApiEndpoint`,
		RunE:  authCommand(getApiEndpoint),
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
		Short: "Find ApiEndpoint",
		Long:  `Find an ApiEndpoint via its displayName (set by operationId from OpenAPI schema)`,
		RunE:  authCommand(findApiEndpoint),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("display-name", "", "Display name of endpoint")
	cmd.MarkFlagRequired("display-name")
	registerPrintableApiCommand(root, cmd)
}

func registerGenerateOpenAPISpecForAPIEndpoint(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "generate-openapi-spec-for-api-endpoint",
		Short: "Generate OpenAPI spec for API endpoint",
		Long:  `Generate an OpenAPI specification for a particular ApiEndpoint`,
		RunE:  authCommand(generateOpenAPISpecForAPIEndpoint),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("api-endpoint-id", "", "API Endpoint ID")
	cmd.MarkFlagRequired("api-endpoint-id")
	cmd.Flags().Bool("diff", false, "Show diff to current version of schema (if available)")
	root.AddCommand(cmd)
}

func registerGeneratePostmanCollectionForAPIEndpoint(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "generate-postman-collection-for-api-endpoint",
		Short: "Generate Postman collection for API endpoint",
		Long:  `Generate a Postman collection for a particular ApiEndpoint`,
		RunE:  authCommand(generatePostmanCollectionForAPIEndpoint),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("api-endpoint-id", "", "API Endpoint ID")
	cmd.MarkFlagRequired("api-endpoint-id")
	root.AddCommand(cmd)
}

func registerRegisterSchema(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "register-schema",
		Short: "Register schema",
		Long:  `Register a schema for a particular apiID and versionID`,
		RunE:  authCommand(registerSchema),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("schema", "", "Path to schema to register")
	cmd.MarkFlagRequired("schema")
	root.AddCommand(cmd)
}

func registerGetSchemas(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-schemas",
		Short: "Get schemas",
		Long:  `Get information about all schemas associated with a particular apiID`,
		RunE:  authCommand(getSchemas),
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
		Short: "Get schema revision",
		Long:  `Get information about a particular schema revision for an Api`,
		RunE:  authCommand(getSchemaRevision),
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
		Short: "Get schema diff",
		Long:  `Get a diff of two schema revisions for an Api`,
		RunE:  authCommand(getSchemaDiff),
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
		Short: "Download latest schema",
		Long:  `Download the latest schema for a particular apiID`,
		RunE:  authCommand(downloadLatestSchema),
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
		Short: "Download schema revision",
		Long:  `Download a particular schema revision for an Api`,
		RunE:  authCommand(downloadSchemaRevision),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	cmd.Flags().String("revision-id", "", "Revision ID")
	cmd.MarkFlagRequired("revision-id")
	root.AddCommand(cmd)
}

func registerGetVersionMetadata(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-version-metadata",
		Short: "Get version metadata",
		Long:  `Get all metadata for a particular apiID and versionID`,
		RunE:  authCommand(getVersionMetadata),
	}
	cmd.Flags().String("api-id", "", "API ID")
	cmd.MarkFlagRequired("api-id")
	cmd.Flags().String("version-id", "", "Version ID")
	cmd.MarkFlagRequired("version-id")
	registerPrintableApiCommand(root, cmd)
}

func registerQueryEventLog(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "query-event-log",
		Short: "Query event log",
		Long:  `Query the event log to retrieve a list of requests`,
		RunE:  authCommand(queryEventLog),
	}
	cmd.Flags().String("filters", "", "JSON string of filters")
	registerPrintableApiCommand(root, cmd)
}

func registerGetRequestFromEventLog(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "get-request-from-event-log",
		Short: "Get request from event log",
		Long:  `Get information about a particular request`,
		RunE:  authCommand(getRequestFromEventLog),
	}
	cmd.Flags().String("request-id", "", "Request ID")
	cmd.MarkFlagRequired("request-id")
	registerPrintableApiCommand(root, cmd)
}

func registerGetValidEmbedAccessTokens(root *cobra.Command) {
	registerPrintableApiCommand(root, &cobra.Command{
		Use:   "get-valid-embed-access-tokens",
		Short: "Get valid embed access tokens",
		Long:  `Get all valid embed access tokens for the current workspace`,
		RunE:  authCommand(getValidEmbedAccessTokens),
	})
}
