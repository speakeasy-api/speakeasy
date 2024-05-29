package suggest

import (
	"bytes"
	"context"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"gopkg.in/yaml.v3"
	"os"
	"slices"
	"strings"
)

func Suggest(ctx context.Context, schemaPath, outPath string, asOverlay bool, style operations.Style, depthStyle operations.DepthStyle) error {
	client, err := auth.GetSDKFromContext(ctx)
	if err != nil {
		return err
	}

	schemaBytes, _, oldDoc, err := schema.LoadDocument(ctx, schemaPath)
	if err != nil {
		return err
	}

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

	/* Update operation IDS and tags/groups */
	newDoc := v3.NewDocument(oldDoc.Model.GoLow()) // Need to keep the old document for overlay comparison
	applySuggestion(ctx, newDoc, res.Suggestion.OperationIds)

	/*
	 * Write the new document or overlay
	 */

	out := os.Stdout
	if outPath != "" {
		out, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
	}

	finalBytes, err := newDoc.Render()
	if err != nil {
		return err
	}

	if asOverlay {
		// Note that newDoc.Index.GetRootNode() should work here, but doesn't
		var y1, y2 yaml.Node
		if err = yaml.NewDecoder(bytes.NewReader(schemaBytes)).Decode(&y1); err != nil {
			return fmt.Errorf("failed to decode source schema bytes: %w", err)
		}
		if err = yaml.NewDecoder(bytes.NewReader(finalBytes)).Decode(&y2); err != nil {
			return fmt.Errorf("failed to decode updated schema bytes: %w", err)
		}

		o, err := overlay.Compare(oldDoc.Model.Info.Title, &y1, y2) // TODO this doesn't work for some reason
		if err != nil {
			return err
		}

		if err := o.Format(out); err != nil {
			return err
		}
	} else {
		if _, err = out.Write(finalBytes); err != nil {
			return err
		}
	}

	/* Regex to override operation IDs ?? */

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
			oldGroupID := operation.Tags[0]
			if len(operation.Tags) > 1 {
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

var changedStyle = styles.Dimmed.Copy().Strikethrough(true)

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

	lhsHeading := styles.Info.Copy().Width(maxWidth).Underline(true).Render("Original")
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

const petstore = `openapi: "3.0.0"
info:
  version: 1.0.0
  title: Swagger Petstore
  license:
    name: MIT
servers:
  - url: http://petstore.swagger.io/v1
paths:
  /pets:
    get:
      summary: List all pets
      operationId: listPets
      tags:
        - pets
      parameters:
        - name: limit
          in: query
          description: How many items to return at one time (max 100)
          required: false
          schema:
            type: integer
            maximum: 100
            format: int32
      responses:
        '200':
          description: A paged array of pets
          headers:
            x-next:
              description: A link to the next page of responses
              schema:
                type: string
          content:
            application/json:    
              schema:
                $ref: "#/components/schemas/Pets"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
    post:
      summary: Create a pet
      operationId: createPets
      tags:
        - pets
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
        required: true
      responses:
        '201':
          description: Null response
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /pets/{petId}:
    get:
      summary: Info for a specific pet
      operationId: showPetById
      tags:
        - pets
      parameters:
        - name: petId
          in: path
          required: true
          description: The id of the pet to retrieve
          schema:
            type: string
      responses:
        '200':
          description: Expected response to a valid request
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Pet"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
components:
  schemas:
    Pet:
      type: object
      required:
        - id
        - name
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        tag:
          type: string
    Pets:
      type: array
      maxItems: 100
      items:
        $ref: "#/components/schemas/Pet"
    Error:
      type: object
      required:
        - code
        - message
      properties:
        code:
          type: integer
          format: int32
        message:
          type: string`
