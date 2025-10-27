package transform

import (
	"context"
	"io"

	"github.com/pb33f/libopenapi/json"
	"github.com/speakeasy-api/jq/pkg/playground"
	"gopkg.in/yaml.v3"
)

// JQSymbolicExecutionDocument applies JQ symbolic execution transformations to an OpenAPI document from a file path.
func JQSymbolicExecutionFromReader(ctx context.Context, in io.Reader, schemaPath string, yamlOut bool, w io.Writer) error {
	schemaBytes, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	newSchema, err := playground.SymbolicExecuteJQ(string(schemaBytes))
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if yamlOut {
		_, err = w.Write([]byte(newSchema))
		return err
	}
	// yaml unmarshal as yaml node
	var node yaml.Node

	if err := yaml.Unmarshal([]byte(newSchema), &node); err != nil {
		return err
	}
	out, err := json.YAMLNodeToJSON(&node, "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}
