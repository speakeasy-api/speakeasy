package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/flag"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
)

type BillingFlags struct {
	Feature string `json:"feature"`
}

var billingCmd = &model.CommandGroup{
	Usage:          "billing",
	Short:          "Manage billing related operations",
	Long:           `Commands for managing billing related operations in Speakeasy.`,
	InteractiveMsg: "What do you want to do?",
	Commands:       []model.Command{activateCmd},
}

var activateCmd = &model.ExecutableCommand[BillingFlags]{
	Usage:        "activate",
	Short:        "Activate a paid feature",
	Long:         `Activate a paid feature in your Speakeasy configuration.`,
	Run:          activateExec,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "feature",
			Description: "feature to activate (e.g. webhooks)",
			Required:    true,
		},
	},
}

func activateExec(ctx context.Context, flags BillingFlags) error {
	logger := log.From(ctx)

	switch flags.Feature {
	case "webhooks":
		return activateWebhooks(ctx)
	default:
		return fmt.Errorf("unsupported feature: %s", flags.Feature)
	}
}

func activateWebhooks(ctx context.Context) error {
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
	logger.Println("https://www.speakeasy.com/docs/customize/webhooks")

	if _, err := charm.NewForm(huh.NewForm(prompt)).ExecuteForm(); err != nil {
		return fmt.Errorf("failed to get user confirmation: %w", err)
	}

	if !confirm {
		logger.Println("Webhook activation cancelled")
		return nil
	}

	// TODO: Send request to activate webhooks set feature flag

	logger.Println("Successfully upgraded - webhooks are now enabled")
	return nil
}
