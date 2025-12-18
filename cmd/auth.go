package cmd

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
	"slices"
	"strings"
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

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch between authenticated workspaces",
	Long:  `Change the default workspace to a different authenticated workspace`,
	RunE:  switchExec,
}

func authInit() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(switchCmd)
	rootCmd.AddCommand(authCmd)
}

func loginExec(cmd *cobra.Command, args []string) error {
	return login(cmd, true)
}

func logoutExec(cmd *cobra.Command, args []string) error {
	return auth.Logout(cmd.Context())
}

func switchExec(cmd *cobra.Command, args []string) error {
	var items []list.Item

	workspaces := config.GetAuthenticatedWorkspaces()
	slices.Sort(workspaces)

	for _, workspace := range workspaces {
		// Always show speakeasy-self at the beginning
		if workspace == "speakeasy-self@speakeasy-self" {
			items = append([]list.Item{interactivity.Item[string]{
				Label: workspace,
				Value: workspace,
			}}, items...)
		} else {
			items = append(items, interactivity.Item[string]{
				Label: workspace,
				Value: workspace,
			})
		}
	}

	selected := interactivity.SelectFrom[string]("Select a workspace to switch to", items)

	parts := strings.Split(selected, "@")
	if len(parts) != 2 {
		return fmt.Errorf("failed to switch workspaces. Unrecognized key format")
	}

	key := config.GetWorkspaceAPIKey(parts[0], parts[1])
	if err := config.ClearSpeakeasyAuthInfo(); err != nil {
		return err
	}
	if err := config.SetSpeakeasyAPIKey(key); err != nil {
		return err
	}

	return login(cmd, false)
}

func login(cmd *cobra.Command, force bool) error {
	authCtx, err := auth.Authenticate(cmd.Context(), force)
	if err != nil {
		return err
	}
	cmd.SetContext(authCtx)
	workspaceID, err := core.GetWorkspaceIDFromContext(authCtx)
	if err != nil {
		return err
	}

	log.From(cmd.Context()).
		WithInteractiveOnly().
		Successf("Authenticated with workspace successfully - %s/workspaces/%s\n", core.GetServerURL(), workspaceID)

	log.From(cmd.Context()).
		WithInteractiveOnly().
		Infof("Review the workspace with `speakeasy status` or create a new target with `speakeasy quickstart`.")

	return nil
}
