package cmd

import (
	"github.com/speakeasy-api/openapi-generation/v2/languageserver"
	"github.com/speakeasy-api/speakeasy/internal/fs"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
)

var languageServerCommand = &cobra.Command{
	Use:    "language-server",
	Short:  "Runs Speakeasy's OpenAPI validator as a Language Server",
	Long:   `Runs Speakeasy's OpenAPI validator as a Language Server, providing a fully compliant LSP backend for OpenAPI linting and validation.`,
	Hidden: true,
}

func languageServerInit(version string) {
	languageServerCommand.RunE = languageServerExec(version)
	rootCmd.AddCommand(languageServerCommand)
}

func languageServerExec(version string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// setup logging to be discarded, it will invalidate the LSP protocol
		logger := log.NewNoop()

		fs := fs.NewFileSystem()

		return languageserver.NewServer(version, logger, fs).Run()
	}
}
