package suggest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	speakeasy "github.com/speakeasy-api/speakeasy-client-sdk-go/v3"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"github.com/speakeasy-api/speakeasy/internal/sdk"
	"gopkg.in/yaml.v3"
)

func Suggest(ctx context.Context, schemaPath, outPath string, asOverlay bool, style operations.Style, depthStyle operations.DepthStyle) error {
	if asOverlay && !isYAML(outPath) {
		return fmt.Errorf("output path must be a YAML or YML file when generating an overlay. Set --overlay=false to write an updated spec")
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	client, err := sdk.InitSDK(speakeasy.WithClient(httpClient))
	if err != nil {
		return err
	}

	schemaBytes, _, oldDoc, err := schema.LoadDocument(ctx, schemaPath)
	if err != nil {
		return err
	}

	stopSpinner := interactivity.StartSpinner("Generating suggestions...")

	/* Get suggestion */
	res, err := client.Suggest.SuggestOperationIDs(ctx, operations.SuggestOperationIDsRequestBody{
		// TODO add these as flags
		Opts: &operations.Opts{
			Style:      style.ToPointer(),
			DepthStyle: depthStyle.ToPointer(),
		},
		Schema: operations.Schema{
			FileName: schemaPath,
			Content:  schemaBytes,
		},
	})
	if err != nil || res.Suggestion == nil {
		return err
	}
	stopSpinner()

	/* Update operation IDS and tags/groups */
	newDoc := v3.NewDocument(oldDoc.Model.GoLow()) // Need to keep the old document for overlay comparison
	applySuggestion(ctx, newDoc, res.Suggestion.OperationIds)

	/*
	 * Write the new document or overlay
	 */

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	finalBytesYAML, err := newDoc.Render()
	if err != nil {
		return err
	}

	if asOverlay {
		// Note that newDoc.Index.GetRootNode() should work here, but doesn't
		var y1, y2 yaml.Node
		if err = yaml.NewDecoder(bytes.NewReader(schemaBytes)).Decode(&y1); err != nil {
			return fmt.Errorf("failed to decode source schema bytes: %w", err)
		}
		if err = yaml.NewDecoder(bytes.NewReader(finalBytesYAML)).Decode(&y2); err != nil {
			return fmt.Errorf("failed to decode updated schema bytes: %w", err)
		}

		o, err := overlay.Compare(oldDoc.Model.Info.Title, &y1, y2) // TODO this doesn't work for some reason
		if err != nil {
			return err
		}

		if err := o.Format(outFile); err != nil {
			return err
		}
	} else {
		// Output yaml if output path is yaml, json if output path is json
		if isYAML(outPath) {
			if _, err = outFile.Write(finalBytesYAML); err != nil {
				return err
			}
		} else {
			finalBytesJSON, err := newDoc.RenderJSON("  ")
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

func applySuggestion(ctx context.Context, model *v3.Document, suggestion map[string][]string) {
	pathItems := model.Paths.PathItems
	var toPrint [][]string

	for pathPair := orderedmap.First(pathItems); pathPair != nil; pathPair = pathPair.Next() {
		operations := pathPair.Value().GetOperations()

		for operationPair := orderedmap.First(operations); operationPair != nil; operationPair = operationPair.Next() {
			operation := operationPair.Value()
			operationID := operation.OperationId
			oldGroupID := "<no_group>"
			if len(operation.Tags) == 1 {
				oldGroupID = operation.Tags[0]
			} else if len(operation.Tags) > 1 {
				oldGroupID = fmt.Sprintf("[%s]", strings.Join(operation.Tags, ", "))
			}

			if newOperationPath, ok := suggestion[operationID]; ok {
				newGroupID := strings.Join(newOperationPath[:len(newOperationPath)-1], ".")
				newOperationID := newOperationPath[len(newOperationPath)-1]

				if !slices.Contains(operation.Tags, newGroupID) {
					operation.Extensions.Set("x-speakeasy-group", buildValueNode(newGroupID))
				}

				if newOperationID != operationID {
					operation.Extensions.Set("x-speakeasy-name-override", buildValueNode(newOperationID))
				}

				toPrint = append(toPrint, []string{oldGroupID, operationID, newGroupID, newOperationID})
			}
		}
	}

	printSuggestions(ctx, toPrint)
}

var changedStyle = styles.Dimmed.Strikethrough(true)

func printSuggestions(ctx context.Context, toPrint [][]string) {
	logger := log.From(ctx)

	maxWidth := 0

	var lhs []string
	var rhs []string

	for _, suggestion := range toPrint {
		oldGroupID, oldOperationID, newGroupID, newOperationID := suggestion[0], suggestion[1], suggestion[2], suggestion[3]

		oldGroupIDStr := styles.Info.Render(oldGroupID)
		oldOperationIDStr := styles.Info.Render(oldOperationID)
		newGroupIDStr := styles.DimmedItalic.Render(newGroupID)
		newOperationIDStr := styles.DimmedItalic.Render(newOperationID)

		if newGroupID != oldGroupID {
			oldGroupIDStr = changedStyle.Render(oldGroupID)
			newGroupIDStr = styles.Success.Render(newGroupID)
		}

		if newOperationID != oldOperationID {
			oldOperationIDStr = changedStyle.Render(oldOperationID)
			newOperationIDStr = styles.Success.Render(newOperationID)
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

func buildValueNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func isYAML(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}
