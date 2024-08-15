package suggest

import (
	"context"
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/auth"
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

func SuggestOperationIDsAndWrite(ctx context.Context, schemaLocation string, asOverlay, yamlOut bool, style shared.Style, depthStyle shared.DepthStyle, w io.Writer) error {
	if asOverlay {
		yamlOut = true
	}

	schemaBytes, _, model, err := schema.LoadDocument(ctx, schemaLocation)
	if err != nil {
		return err
	}

	stopSpinner := interactivity.StartSpinner("Generating suggestions...")

	updates, overlay, err := SuggestOperationIDs(ctx, schemaBytes, model.Model, style, depthStyle)

	stopSpinner()

	if err != nil {
		return err
	}

	printSuggestions(ctx, updates)

	/*
	 * Write the new document or overlay
	 */
	if asOverlay {
		if err := overlay.Format(w); err != nil {
			return err
		}
	} else {
		root := model.Index.GetRootNode()
		if err := overlay.ApplyTo(root); err != nil {
			return err
		}

		finalBytes, err := overlayUtil.Render(root, schemaLocation, yamlOut)
		if err != nil {
			return err
		}

		if _, err = w.Write(finalBytes); err != nil {
			return err
		}
	}

	return nil
}

func SuggestOperationIDs(ctx context.Context, schema []byte, model v3.Document, style shared.Style, depthStyle shared.DepthStyle) ([]suggestions.OperationUpdate, *overlay.Overlay, error) {
	httpClient := &http.Client{Timeout: 5 * time.Minute}
	severURL := speakeasy.ServerList[speakeasy.ServerProd]
	if strings.Contains(auth.GetServerURL(), "localhost") {
		severURL = "http://localhost:35291"
	}
	client, err := sdk.InitSDK(
		speakeasy.WithClient(httpClient),
		speakeasy.WithServerURL(severURL),
	)
	if err != nil {
		return nil, nil, err
	}

	/* Get suggestion */
	res, err := client.Suggest.SuggestOperationIDs(ctx, operations.SuggestOperationIDsRequest{
		XSessionID: "unused",
		RequestBody: operations.SuggestOperationIDsRequestBody{
			Opts: &shared.SuggestOperationIDsOpts{
				Style:      style.ToPointer(),
				DepthStyle: depthStyle.ToPointer(),
			},
			Schema: operations.Schema{
				FileName: "openapi.yaml",
				Content:  schema,
			},
		},
	})
	if err != nil || res.SuggestedOperationIDs == nil {
		return nil, nil, err
	}

	/* Convert result to overlay */
	suggestion := suggestions.MakeOperationIDs(res.SuggestedOperationIDs.OperationIds)
	updates, overlay := suggestion.AsOverlay(model)

	return updates, &overlay, nil
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
