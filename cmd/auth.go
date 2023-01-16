package cmd

import (
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate the CLI",
	Long:  `The "authenticate" command allows control over the authentication of the CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate the CLI",
	Long:  `The "login" command authenticates the CLI for use with the Speakeasy Platform.`,
	RunE:  loginExec,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout of the CLI",
	Long:  `The "logout" command removes authentication from the CLI.`,
	RunE:  logoutExec,
}

func authInit() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(authCmd)
}

func loginExec(cmd *cobra.Command, args []string) error {
	return auth.Authenticate(true)
}

func logoutExec(cmd *cobra.Command, args []string) error {
	return auth.Logout()
}
