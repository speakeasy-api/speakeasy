package transform

import (
	"context"
	"fmt"
	"io"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"gopkg.in/yaml.v3"
)

func FormatDocument(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: Format,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func FormatFromReader(ctx context.Context, schema io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	return transformer[interface{}]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: Format,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func Format(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], _ interface{}) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	root := model.Index.GetRootNode()

	// Define the desired order of keys for different levels
	rootOrder := []string{
		"openapi",
		"info",
		"security",
		"servers",
		"paths",
		"components",
		"tags",
	}

	componentsOrder := []string{
		"securitySchemes",
		"schemas",
		"responses",
		"requestBodies",
		"parameters",
		"examples",
		"headers",
		"links",
		"callbacks",
	}

	// Walk through and reorder all mapping nodes
	walkAndReorderNodes(ctx, root, rootOrder, componentsOrder)

	// Render and reload the document to ensure that the changes are reflected in the model
	updatedDoc, err := yaml.Marshal(root)
	if err != nil {
		return doc, model, fmt.Errorf("failed to marshal document: %w", err)
	}

	docNew, model, err := openapi.Load(updatedDoc, doc.GetConfiguration().BasePath)
	if err != nil {
		return doc, model, fmt.Errorf("failed to reload document: %w", err)
	}

	return *docNew, model, nil
}

func walkAndReorderNodes(ctx context.Context, node *yaml.Node, rootOrder, componentsOrder []string) {
	if node == nil {
		return
	}

	if node.Kind == yaml.MappingNode {
		// Determine which order to use based on the context
		orderToUse := rootOrder
		if len(node.Content) > 0 {
			// Check if this is the components section
			if node.Content[0].Value == "components" {
				orderToUse = componentsOrder
			}
		}

		reorderYAMLNode(node, orderToUse)
	}

	// Recursively process all child nodes
	for _, child := range node.Content {
		walkAndReorderNodes(ctx, child, rootOrder, componentsOrder)
	}
}

func reorderYAMLNode(node *yaml.Node, order []string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("node is not a map")
	}

	// Create a map to hold key-value pairs by key
	kvMap := make(map[string]*yaml.Node)
	// Keep track of original keys to handle unknown ones
	originalKeys := make([]string, 0)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		kvMap[keyNode.Value] = valueNode
		originalKeys = append(originalKeys, keyNode.Value)
	}

	// Clear the current Content slice
	node.Content = []*yaml.Node{}

	// First, append key-value pairs in the specified order
	for _, key := range order {
		if valueNode, ok := kvMap[key]; ok {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}, valueNode)
			// Remove from kvMap to track what's been processed
			delete(kvMap, key)
		}
	}

	// Then append any remaining keys that weren't in the order slice
	for _, key := range originalKeys {
		if valueNode, ok := kvMap[key]; ok {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}, valueNode)
		}
	}

	return nil
}
