package cmd

import (
	"fmt"
	"os"

	"github.com/speakeasy-api/huh"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/spf13/cobra"
)

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Manage billing related operations",
	Long:  `Commands for managing billing related operations in Speakeasy.`,
}

var activateWebhooksCmd = &cobra.Command{
	Use:   "activate-webhooks",
	Short: "Activate webhooks in the SDK generation configuration",
	Long:  `Search for and update the SDK generation configuration file to activate webhooks for the configured language.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := log.From(cmd.Context())

		// Show warning and confirmation prompt
		logger.Println(styles.RenderWarningMessage(
			"Webhooks are a paid feature",
			"Activating webhooks will enable billing for this feature",
		))

		confirm := false
		prompt := charm.NewBranchPrompt(
			"Would you like to proceed with activating webhooks?",
			"This will enable billing for the webhooks feature",
			&confirm,
		)

		logger.Println("For more information, see:")
		logger.Println("https://www.speakeasyapi.dev/docs/advanced-setup/webhooks")

		if _, err := charm.NewForm(huh.NewForm(prompt)).ExecuteForm(); err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}

		if !confirm {
			logger.Println("Webhook activation cancelled")
			return nil
		}

		// Load the SDK generation config
		workingDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		cfg, err := sdkGenConfig.Load(workingDir)
		if err != nil {
			return fmt.Errorf("failed to load SDK generation config: %w", err)
		}

		// Track if we found any languages to update
		updated := false

		// Enable webhooks for each configured language
		for _, lang := range cfg.Config.Languages {
			// Create webhooks config if it doesn't exist
			if lang.Cfg["webhooks"] == nil {
				lang.Cfg["webhooks"] = map[string]any{}
			}

			// Enable webhooks
			lang.Cfg["webhooks"].(map[string]any)["enabled"] = true
			updated = true
		}

		if !updated {
			return fmt.Errorf("no supported language configuration found in gen.yaml")
		}

		// Save the updated config
		if err := sdkGenConfig.SaveConfig(workingDir, cfg.Config); err != nil {
			return fmt.Errorf("failed to save SDK generation config: %w", err)
		}

		logger.Println("Successfully upgraded - webhooks are now enabled")
		logger.Println("For more information, see:")
		logger.Println("https://www.speakeasyapi.dev/docs/advanced-setup/webhooks")
		return nil
	},
}

func init() {
	billingCmd.AddCommand(activateWebhooksCmd)
	rootCmd.AddCommand(billingCmd)
}
