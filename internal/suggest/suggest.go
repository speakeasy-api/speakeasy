package suggest

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/log"
	overlayUtil "github.com/speakeasy-api/speakeasy/internal/overlay"
	"github.com/speakeasy-api/speakeasy/internal/schema"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
)

func Suggest(ctx context.Context, schemaLocation, outPath string, asOverlay bool, style shared.Style, depthStyle shared.DepthStyle) error {
	if asOverlay && !utils.HasYAMLExt(outPath) {
		return fmt.Errorf("output path must be a YAML or YML file when generating an overlay. Set --overlay=false to write an updated spec")
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	client, err := sdk.InitSDK(speakeasy.WithClient(httpClient))
	if err != nil {
		return err
	}

	schemaBytes, _, model, err := schema.LoadDocument(ctx, schemaLocation)
	if err != nil {
		return err
	}

	stopSpinner := interactivity.StartSpinner("Generating suggestions...")

	/* Get suggestion */
	res, err := client.Suggest.SuggestOperationIDs(ctx, operations.SuggestOperationIDsRequest{
		XSessionID: "unused",
		RequestBody: operations.SuggestOperationIDsRequestBody{
			Opts: &shared.SuggestOperationIDsOpts{
				Style:      style.ToPointer(),
				DepthStyle: depthStyle.ToPointer(),
			},
			Schema: operations.Schema{
				FileName: schemaLocation,
				Content:  schemaBytes,
			},
		},
	})
	if err != nil || res.SuggestedOperationIDs == nil {
		return err
	}
	stopSpinner()

	/* Update operation IDS and tags/groups */
	suggestion := suggestions.MakeOperationIDs(res.SuggestedOperationIDs.OperationIds)
	updates, overlay := suggestion.AsOverlay(model.Model)
	printSuggestions(ctx, updates)

	/*
	 * Write the new document or overlay
	 */

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if asOverlay {
		if err := overlay.Format(outFile); err != nil {
			return err
		}
	} else {
		root := model.Index.GetRootNode()
		if err := overlay.ApplyTo(root); err != nil {
			return err
		}

		finalBytes, err := overlayUtil.Render(root, schemaLocation, utils.HasYAMLExt(outPath))
		if err != nil {
			return err
		}

		if _, err = outFile.Write(finalBytes); err != nil {
			return err
		}
	}

	return nil
}

var changedStyle = styles.Dimmed.Strikethrough(true)

func printSuggestions(ctx context.Context, updates []suggestions.OperationUpdate) {
	logger := log.From(ctx)

	maxWidth := 0

	var lhs []string
	var rhs []string

	for _, update := range updates {
		oldGroupIDStr := styles.Info.Render(update.OldGroupID)
		oldOperationIDStr := styles.Info.Render(update.OldOperationID)
		newGroupIDStr := styles.DimmedItalic.Render(update.NewGroupID)
		newOperationIDStr := styles.DimmedItalic.Render(update.NewOperationID)

		if update.NewGroupID != update.OldGroupID {
			oldGroupIDStr = changedStyle.Render(update.OldGroupID)
			newGroupIDStr = styles.Success.Render(update.NewGroupID)
		}

		if update.NewOperationID != update.OldOperationID {
			oldOperationIDStr = changedStyle.Render(update.OldOperationID)
			newOperationIDStr = styles.Success.Render(update.NewOperationID)
		}

		l := fmt.Sprintf("%s.%s", oldGroupIDStr, oldOperationIDStr)
		lhs = append(lhs, l)

		rhs = append(rhs, fmt.Sprintf("%s.%s", newGroupIDStr, newOperationIDStr))

		if w := lipgloss.Width(l); w > maxWidth {
			maxWidth = w
		}
	}

	lhsHeading := styles.Info.Width(maxWidth).Underline(true).Render("Original")
	rhsHeading := styles.Success.Underline(true).Render("Suggested")
	logger.Printf("%s    %s", lhsHeading, rhsHeading)

	arrow := styles.HeavilyEmphasized.Render("->")
	for i := range lhs {
		l := lipgloss.NewStyle().Width(maxWidth).Render(lhs[i])
		logger.Printf("%s %s %s", l, arrow, rhs[i])
	}
}
