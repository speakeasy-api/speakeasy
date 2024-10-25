package schemas

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pb33f/libopenapi/json"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

// Format reformats a document to the desired output format while preserving key ordering
// Can be used to convert output types, or improve readability (e.g. prettifying single-line documents)
func Format(ctx context.Context, schemaPath string, yamlOut bool) ([]byte, error) {
	_, _, model, err := LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return Render(model.Index.GetRootNode(), schemaPath, yamlOut)
}

func Render(y *yaml.Node, schemaPath string, yamlOut bool) ([]byte, error) {
	yamlIn := utils.HasYAMLExt(schemaPath)
	return RenderDocument(y, schemaPath, yamlIn, yamlOut)
}

// RenderDocument - schemaPath can be unset if the docuemnt does not need reference resolution
func RenderDocument(y *yaml.Node, schemaPath string, yamlIn bool, yamlOut bool) ([]byte, error) {
	if yamlIn && yamlOut {
		var res bytes.Buffer
		encoder := yaml.NewEncoder(&res)
		// Note: would love to make this generic but the indentation information isn't in go-yaml nodes
		// https://github.com/go-yaml/yaml/issues/899
		encoder.SetIndent(2)
		if err := encoder.Encode(y); err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		return res.Bytes(), nil
	}

	// Preserves key ordering
	specBytes, err := json.YAMLNodeToJSON(y, "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	if yamlOut {
		// Use libopenapi to convert JSON to YAML to preserve key ordering
		_, model, err := openapi.Load(specBytes, schemaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load document: %w", err)
		}

		yamlBytes := model.Model.RenderWithIndention(2)

		return yamlBytes, nil
	} else {
		return specBytes, nil
	}
}
