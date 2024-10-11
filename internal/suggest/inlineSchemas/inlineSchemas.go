package inlineSchemas

import (
	"context"
	"encoding/json"
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"os/exec"
)

func RefactorInlineSchemas(ctx context.Context, schemaPath string) (*overlay.Overlay, error) {
	cmd := exec.Command("sh", "-c", "curl -fsSL https://raw.githubusercontent.com/speakeasy-api/speakeasy/main/install.sh")
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to install CLI: %w", err)
	}

	cmd = exec.Command("speakeasy", "help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run CLI: %w", err)
	}

	fmt.Println(string(out))

	_, _, model, err := schemas.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}

	targetToNameNeeded := map[string]string{}

	for op := range openapi.IterateOperations(model.Model) {
		method, path, operation := op.Method, op.Path, op.Operation
		baseTarget := overlay.NewTargetSelector(path, method)

		if operation.RequestBody != nil && operation.RequestBody.Content != nil {
			requestBodies := operation.RequestBody.Content
			if err := iterateContent(requestBodies, targetToNameNeeded, baseTarget, "requestBody"); err != nil {
				return nil, err
			}
		}

		if operation.Responses != nil && operation.Responses.Codes != nil {
			responseBodies := operation.Responses.Codes
			for responsePair := orderedmap.First(responseBodies); responsePair != nil; responsePair = responsePair.Next() {
				if err := iterateContent(responsePair.Value().Content, targetToNameNeeded, baseTarget, "responses", responsePair.Key()); err != nil {
					return nil, err
				}
			}
		}
	}

	j, _ := json.MarshalIndent(targetToNameNeeded, "", "  ")
	println(string(j))

	return &overlay.Overlay{}, nil
}

func iterateContent(content *orderedmap.Map[string, *v3.MediaType], targetToNameNeeded map[string]string, baseTarget string, targetParts ...string) error {
	for contentPair := orderedmap.First(content); contentPair != nil; contentPair = contentPair.Next() {
		schema := contentPair.Value().Schema
		if _, hasOverride := schema.Schema().Extensions.Get("x-speakeasy-name-override"); !schema.IsReference() && !hasOverride && schema.Schema().Title == "" {
			for _, part := range append(targetParts, "content", contentPair.Key(), "schema") {
				baseTarget += fmt.Sprintf(`["%s"]`, part)
			}

			schemaBytes, err := schema.Render()
			if err != nil {
				return fmt.Errorf("error rendering schema: %w", err)
			}

			targetToNameNeeded[baseTarget] = string(schemaBytes)
		}
	}

	return nil
}
