package transform

import (
	"context"
	"fmt"
	"io"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
)

func RemoveUnused(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: RemoveOrphans,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func RemoveUnusedFromReader(ctx context.Context, schema io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	return transformer[interface{}]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: RemoveOrphans,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func RemoveOrphans(ctx context.Context, doc *openapi.OpenAPI, _ interface{}) (*openapi.OpenAPI, error) {
	logger := log.From(ctx)

	if doc.Components == nil {
		return doc, nil
	}

	// Sync to ensure YAML is up to date, then collect all refs from YAML
	if err := syncDoc(ctx, doc); err != nil {
		return doc, err
	}

	root := doc.GetCore().GetRootNode()

	// Collect all $ref values in the document
	allRefs := collectAllRefs(root)

	// Also collect security requirement references
	securityRefs := collectSecurityRefs(doc)

	anyRemoved := false

	// Remove unused schemas
	if doc.Components.Schemas != nil {
		var toDelete []string
		for name := range doc.Components.Schemas.All() {
			ref := fmt.Sprintf("#/components/schemas/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				logger.Printf("dropped #/components/schemas/%s\n", name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.Schemas.Delete(key)
		}
	}

	// Remove unused responses
	if doc.Components.Responses != nil {
		var toDelete []string
		for name := range doc.Components.Responses.All() {
			ref := fmt.Sprintf("#/components/responses/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				logger.Printf("dropped #/components/responses/%s\n", name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.Responses.Delete(key)
		}
	}

	// Remove unused parameters
	if doc.Components.Parameters != nil {
		var toDelete []string
		for name := range doc.Components.Parameters.All() {
			ref := fmt.Sprintf("#/components/parameters/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.Parameters.Delete(key)
		}
	}

	// Remove unused examples
	if doc.Components.Examples != nil {
		var toDelete []string
		for name := range doc.Components.Examples.All() {
			ref := fmt.Sprintf("#/components/examples/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.Examples.Delete(key)
		}
	}

	// Remove unused request bodies
	if doc.Components.RequestBodies != nil {
		var toDelete []string
		for name := range doc.Components.RequestBodies.All() {
			ref := fmt.Sprintf("#/components/requestBodies/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.RequestBodies.Delete(key)
		}
	}

	// Remove unused headers
	if doc.Components.Headers != nil {
		var toDelete []string
		for name := range doc.Components.Headers.All() {
			ref := fmt.Sprintf("#/components/headers/%s", name)
			if !allRefs[ref] {
				toDelete = append(toDelete, name)
				anyRemoved = true
			}
		}
		for _, key := range toDelete {
			doc.Components.Headers.Delete(key)
		}
	}

	// Keep security schemes that are referenced in security requirements
	// (don't remove unused security schemes - they might be intentionally defined)
	_ = securityRefs

	if anyRemoved {
		// Reload and recurse to handle transitive removals
		if err := syncDoc(ctx, doc); err != nil {
			return doc, err
		}
		root = doc.GetCore().GetRootNode()
		newDoc, err := reloadFromYAML(ctx, root)
		if err != nil {
			return doc, err
		}
		return RemoveOrphans(ctx, newDoc, nil)
	}

	return doc, nil
}

// collectAllRefs walks the YAML tree and collects all $ref values
func collectAllRefs(node *yaml.Node) map[string]bool {
	refs := make(map[string]bool)
	collectRefsRecursive(node, refs)
	return refs
}

func collectRefsRecursive(node *yaml.Node, refs map[string]bool) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			collectRefsRecursive(child, refs)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			if keyNode.Value == "$ref" && valueNode.Kind == yaml.ScalarNode {
				refs[valueNode.Value] = true
			}

			// Also check for allOf, anyOf, oneOf which contain refs
			collectRefsRecursive(valueNode, refs)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			collectRefsRecursive(child, refs)
		}
	}
}

// collectSecurityRefs collects security scheme names from security requirements
func collectSecurityRefs(doc *openapi.OpenAPI) map[string]bool {
	refs := make(map[string]bool)

	// Global security
	for _, req := range doc.Security {
		if req != nil {
			for name := range req.All() {
				refs[name] = true
			}
		}
	}

	// Operation-level security
	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.All() {
			if pathItem == nil || pathItem.Object == nil {
				continue
			}
			for _, op := range pathItem.Object.All() {
				if op == nil {
					continue
				}
				for _, req := range op.Security {
					if req != nil {
						for name := range req.All() {
							refs[name] = true
						}
					}
				}
			}
		}
	}

	return refs
}
