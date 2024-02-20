package cmd

import (
	"context"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate the CLI",
	Long:  `The "authenticate" command allows control over the authentication of the CLI.`,
	RunE:  interactivity.InteractiveRunFn("Choose an option:"),
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
	err := events.Telemetry(cmd.Context(), shared.InteractionTypeAuthenticate, func(ctx context.Context, event *shared.CliEvent) error {
		authCtx, err := auth.Authenticate(cmd.Context(), true)
		if err != nil {
			return err
		}
		cmd.SetContext(authCtx)
		workspaceID, err := core.GetWorkspaceIDFromContext(authCtx)
		if err != nil {
			return err
		}
		event.WorkspaceID = workspaceID
		log.From(cmd.Context()).
			WithInteractiveOnly().
			Successf("Authenticated with workspace successfully - %s/workspaces/%s\n", core.GetServerURL(), workspaceID)

		return nil
	})

	// Manually Flush the events because Telemetry will have been initially called with an unauthorized context, now we've authenticated we can send the events
	if err == nil {
		_ = events.FlushEvents(cmd.Context())
	}
	return err
}

func logoutExec(cmd *cobra.Command, args []string) error {
	return auth.Logout(cmd.Context())
}
