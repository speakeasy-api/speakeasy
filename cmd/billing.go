package cmd

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
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
	Long:         `Activate a paid feature in your Speakeasy workspace.`,
	Run:          activateExec,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.EnumFlag{
			Name:        "feature",
			Shorthand:   "f",
			Description: "feature to activate (e.g. webhooks)",
			Required:    true,
			AllowedValues: []string{
				"webhooks",
			},
		},
	},
}

func activateExec(ctx context.Context, flags BillingFlags) error {
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
	sdk, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sdk from context: %w", err)
	}

	if _, err = sdk.Workspaces.SetFeatureFlags(ctx, shared.WorkspaceFeatureFlagRequest{
		FeatureFlags: []shared.WorkspaceFeatureFlag{
			shared.WorkspaceFeatureFlagWebhooks,
		},
	}); err != nil {
		return fmt.Errorf("failed to set feature flags: %w", err)
	}

	logger.Println("Successfully upgraded - webhooks are now enabled")
	return nil
}
