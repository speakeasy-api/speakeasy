package cmd

import (
	"os"

	"github.com/speakeasy-api/speakeasy-proxy/pkg/proxy"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:    "proxy",
	Short:  "Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities",
	Long:   `Proxy provides a reverse-proxy for debugging and testing Speakeasy's Traffic Capture capabilities`,
	RunE:   proxyExec,
	Hidden: true,
}

func proxyInit() {
	proxyCmd.Flags().StringP("downstream", "d", "", "the downstream base url to proxy traffic to")
	proxyCmd.MarkFlagRequired("downstream")
	proxyCmd.Flags().StringP("api-id", "a", "", "the API ID to send captured traffic to")
	proxyCmd.MarkFlagRequired("api-id")
	proxyCmd.Flags().StringP("version-id", "v", "", "the Version ID to send captured traffic to")
	proxyCmd.MarkFlagRequired("version-id")
	proxyCmd.Flags().StringP("schema", "s", "", "path to an openapi document that can be used to match incoming traffic to API endpoints")
	proxyCmd.Flags().StringP("port", "p", "3333", "port to run the proxy on")

	rootCmd.AddCommand(proxyCmd)
}

func proxyExec(cmd *cobra.Command, args []string) error {
	if err := auth.Authenticate(cmd.Context(), false); err != nil {
		return err
	}

	downstreamBaseURL, err := cmd.Flags().GetString("downstream")
	if err != nil {
		return err
	}

	apiID, err := cmd.Flags().GetString("api-id")
	if err != nil {
		return err
	}

	versionID, err := cmd.Flags().GetString("version-id")
	if err != nil {
		return err
	}

	port, err := cmd.Flags().GetString("port")
	if err != nil {
		return err
	}

	schema, err := cmd.Flags().GetString("schema")
	if err != nil {
		return err
	}

	var doc []byte

	if schema != "" {
		doc, err = os.ReadFile(schema)
		if err != nil {
			return err
		}
	}

	apiKey, _ := config.GetSpeakeasyAPIKey()

	return proxy.StartProxy(proxy.ProxyConfig{
		DownstreamBaseURL: downstreamBaseURL,
		APIKey:            apiKey,
		Port:              port,
		ApiID:             apiID,
		VersionID:         versionID,
		OpenAPIDocument:   doc,
	})
}
