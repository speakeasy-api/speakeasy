package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/huh"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
)

type BillingFlags struct{}

type ActivateWebhooksFlags struct{}

var billingCmd = &model.CommandGroup{
	Usage:          "billing",
	Short:          "Manage billing related operations",
	Long:           `Commands for managing billing related operations in Speakeasy.`,
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{activateWebhooksCmd},
}

var activateWebhooksCmd = &model.ExecutableCommand[ActivateWebhooksFlags]{
	Usage:        "activate-webhooks",
	Short:        "Activate webhooks in the SDK generation configuration",
	Long:         `Search for and update the SDK generation configuration file to activate webhooks for the configured language.`,
	Run:          activateWebhooksExec,
	RequiresAuth: true,
}

func activateWebhooksExec(ctx context.Context, flags ActivateWebhooksFlags) error {
	logger := log.From(ctx)

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

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	cfg, err := sdkGenConfig.Load(workingDir)
	if err != nil {
		return fmt.Errorf("failed to load SDK generation config: %w", err)
	}

	updated := false

	for _, lang := range cfg.Config.Languages {
		if lang.Cfg["webhooks"] == nil {
			lang.Cfg["webhooks"] = map[string]any{}
		}

		lang.Cfg["webhooks"].(map[string]any)["enabled"] = true
		updated = true
	}

	if !updated {
		return fmt.Errorf("no supported language configuration found in gen.yaml")
	}

	if err := sdkGenConfig.SaveConfig(workingDir, cfg.Config); err != nil {
		return fmt.Errorf("failed to save SDK generation config: %w", err)
	}

	logger.Println("Successfully upgraded - webhooks are now enabled")
	logger.Println("For more information, see:")
	logger.Println("https://www.speakeasyapi.dev/docs/advanced-setup/webhooks")
	return nil
}
