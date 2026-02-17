package transform

import (
	"context"
	"fmt"
	"io"

	"github.com/speakeasy-api/openapi/openapi"
	"gopkg.in/yaml.v3"
)

type normalizeArgs struct {
	schemaPath string
	NormalizeOptions
}

func NormalizeDocument(ctx context.Context, schemaPath string, prefixItems, yamlOut bool, w io.Writer) error {
	return transformer[normalizeArgs]{
		schemaPath:  schemaPath,
		transformFn: Normalize,
		w:           w,
		jsonOut:     !yamlOut,
		args: normalizeArgs{
			schemaPath: schemaPath,
			NormalizeOptions: NormalizeOptions{
				PrefixItems: prefixItems,
			},
		},
	}.Do(ctx)
}

func NormalizeFromReader(ctx context.Context, schema io.Reader, schemaPath string, prefixItems bool, w io.Writer, yamlOut bool) error {
	return transformer[normalizeArgs]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: Normalize,
		w:           w,
		jsonOut:     !yamlOut,
		args: normalizeArgs{
			schemaPath: schemaPath,
			NormalizeOptions: NormalizeOptions{
				PrefixItems: prefixItems,
			},
		},
	}.Do(ctx)
}

func Normalize(ctx context.Context, doc *openapi.OpenAPI, args normalizeArgs) (*openapi.OpenAPI, error) {
	// Sync to get current YAML nodes
	if err := syncDoc(ctx, doc); err != nil {
		return doc, fmt.Errorf("failed to sync document: %w", err)
	}

	root := doc.GetCore().GetRootNode()

	walkAndNormalizeDocument(root, args.NormalizeOptions)

	// Reload from modified YAML to create fresh document
	newDoc, err := reloadFromYAML(ctx, root)
	if err != nil {
		return doc, fmt.Errorf("failed to reload document: %w", err)
	}

	return newDoc, nil
}

type NormalizeOptions struct {
	PrefixItems bool
}

func walkAndNormalizeDocument(node *yaml.Node, options NormalizeOptions) {

	switch node.Kind {
	case yaml.MappingNode, yaml.DocumentNode:
		for _, child := range node.Content {
			if options.PrefixItems && child.Value == "prefixItems" {
				normalizePrefixItems(node)
				break
			}
			walkAndNormalizeDocument(child, options)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			walkAndNormalizeDocument(child, options)
		}
	}
}

func removeKeys(mapNode *yaml.Node, keys ...string) {
	keysToRemove := make(map[string]struct{})
	for _, k := range keys {
		keysToRemove[k] = struct{}{}
	}

	newContent := make([]*yaml.Node, 0, len(mapNode.Content))
	for i := 0; i < len(mapNode.Content); i += 2 {
		kNode := mapNode.Content[i]
		vNode := mapNode.Content[i+1]
		if _, found := keysToRemove[kNode.Value]; !found {
			// Keep this pair
			newContent = append(newContent, kNode, vNode)
		}
	}
	mapNode.Content = newContent
}

func addKeyValue(mapNode *yaml.Node, key, value string) {
	// Create a key node
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}

	// Create a value node
	valNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}

	// Append them to the map content
	mapNode.Content = append(mapNode.Content, keyNode, valNode)
}

// Take the given yaml OpenAPI node, remove the prefixItems key change the type key to string, remove the minItems and maxItems keys
func normalizePrefixItems(node *yaml.Node) {

	keysToRemove := []string{"prefixItems", "minItems", "maxItems", "type"}
	removeKeys(node, keysToRemove...)

	addKeyValue(node, "type", "string")
}
