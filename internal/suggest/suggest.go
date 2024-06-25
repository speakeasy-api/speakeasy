package suggest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schema"

	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
)

func Suggest(ctx context.Context, schemaLocation, outPath string, asOverlay bool, style shared.Style, depthStyle shared.DepthStyle) error {
	if asOverlay && !isYAML(outPath) {
		return fmt.Errorf("output path must be a YAML or YML file when generating an overlay. Set --overlay=false to write an updated spec")
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	client, err := sdk.InitSDK(speakeasy.WithClient(httpClient))
	if err != nil {
		return err
	}

	schemaBytes, _, _, err := schema.LoadDocument(ctx, schemaLocation)
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
	_, newDoc, err := openapi.Load(schemaBytes) // Need to keep the old document for overlay comparison
	if err != nil {
		return err
	}

	suggestion := suggestions.MakeOperationIDs(res.SuggestedOperationIDs.OperationIds)
	updates := suggestion.Apply(newDoc.Model)
	printSuggestions(ctx, updates)

	/*
	 * Write the new document or overlay
	 */

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	finalBytesYAML, err := newDoc.Model.Render()
	if err != nil {
		return err
	}

	if asOverlay {
		if err = openapi.WriteOverlay(schemaBytes, finalBytesYAML, outFile); err != nil {
			return err
		}
	} else {
		// Output yaml if output path is yaml, json if output path is json
		if isYAML(outPath) {
			if _, err = outFile.Write(finalBytesYAML); err != nil {
				return err
			}
		} else {
			finalBytesJSON, err := newDoc.Model.RenderJSON("  ")
			if err != nil {
				return err
			}

			if _, err = outFile.Write(finalBytesJSON); err != nil {
				return err
			}
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

func isYAML(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}
