package schemas

import (
	"bytes"
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi/json"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/yml"
	"gopkg.in/yaml.v3"
)

// Format reformats a document to the desired output format while preserving key ordering
// Can be used to convert output types, or improve readability (e.g. prettifying single-line documents)
func Format(ctx context.Context, schemaPath string, yamlOut bool) ([]byte, error) {
	_, doc, err := LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	cfg := doc.GetCore().GetConfig()
	if yamlOut {
		cfg.OutputFormat = yml.OutputFormatYAML
	} else {
		cfg.OutputFormat = yml.OutputFormatJSON
	}
	ctx = yml.ContextWithConfig(ctx, cfg)

	var buf bytes.Buffer
	if err := openapi.Marshal(ctx, doc, &buf); err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	return buf.Bytes(), nil
}

func Render(node *yaml.Node, schemaPath string, yamlOut bool) ([]byte, error) {
	if yamlOut {
		var res bytes.Buffer
		encoder := yaml.NewEncoder(&res)
		// Note: would love to make this generic but the indentation information isn't in go-yaml nodes
		// https://github.com/go-yaml/yaml/issues/899
		encoder.SetIndent(2)
		if err := encoder.Encode(node); err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		return res.Bytes(), nil
	}

	var buf bytes.Buffer
	if err := json.YAMLToJSON(node, 2, &buf); err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	return buf.Bytes(), nil
}
