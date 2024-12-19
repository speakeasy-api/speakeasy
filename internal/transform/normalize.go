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

type normalizeArgs struct {
	schemaPath       string
	normalizeOptions NormalizeOptions
}

func NormalizeDocument(ctx context.Context, schemaPath string, normalizeOptions NormalizeOptions, yamlOut bool, w io.Writer) error {
	return transformer[normalizeArgs]{
		schemaPath:  schemaPath,
		transformFn: Normalize,
		w:           w,
		jsonOut:     !yamlOut,
		args: normalizeArgs{
			schemaPath:       schemaPath,
			normalizeOptions: normalizeOptions,
		},
	}.Do(ctx)
}

func NormalizeFromReader(ctx context.Context, schema io.Reader, schemaPath string, normalizeOptions NormalizeOptions, w io.Writer, yamlOut bool) error {
	return transformer[normalizeArgs]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: Normalize,
		w:           w,
		jsonOut:     !yamlOut,
		args: normalizeArgs{
			schemaPath:       schemaPath,
			normalizeOptions: normalizeOptions,
		},
	}.Do(ctx)
}

func Normalize(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], args normalizeArgs) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	root := model.Index.GetRootNode()

	walkAndNormalizeDocument(root, args.normalizeOptions)

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
