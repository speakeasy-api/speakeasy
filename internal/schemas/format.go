package schemas

import (
	"bytes"
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi/json"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

// Format reformats a document to the desired output format while preserving key ordering
// Can be used to convert output types, or improve readability (e.g. prettifying single-line documents)
func Format(ctx context.Context, schemaPath string, yamlOut bool) ([]byte, error) {
	schemaBytes, _, err := LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(schemaBytes, &root); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return Render(&root, schemaPath, yamlOut)
}

func Render(y *yaml.Node, schemaPath string, yamlOut bool) ([]byte, error) {
	yamlIn := utils.HasYAMLExt(schemaPath)
	return RenderDocument(y, schemaPath, yamlIn, yamlOut)
}

// RenderDocument - schemaPath can be unset if the docuemnt does not need reference resolution
func RenderDocument(y *yaml.Node, schemaPath string, yamlIn bool, yamlOut bool) ([]byte, error) {
	if yamlOut {
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

	// Convert to JSON, preserving key ordering
	var buf bytes.Buffer
	if err := json.YAMLToJSON(y, 2, &buf); err != nil {
		return nil, fmt.Errorf("failed to convert to JSON: %w", err)
	}
	return buf.Bytes(), nil
}
