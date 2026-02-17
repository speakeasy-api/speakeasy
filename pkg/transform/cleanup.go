package transform

import (
	"context"
	"io"
	"strings"
	"unicode"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
)

func CleanupDocument(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: Cleanup,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func CleanupFromReader(ctx context.Context, schema io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	return transformer[interface{}]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: Cleanup,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func Cleanup(ctx context.Context, doc *openapi.OpenAPI, _ interface{}) (*openapi.OpenAPI, error) {
	if doc.Paths == nil {
		return doc, nil
	}

	var pathsToDelete []string

	for path, pathItem := range doc.Paths.All() {
		// Check if the path item has any operations
		hasOperations := false
		if pathItem != nil && pathItem.Object != nil {
			hasOperations = pathItem.Object.Len() > 0
		}
		if !hasOperations {
			pathsToDelete = append(pathsToDelete, path)
		}
	}

	for _, path := range pathsToDelete {
		log.From(ctx).Printf("Dropped empty path: %s\n", path)
		doc.Paths.Delete(path)
	}

	// Sync to apply path deletions to YAML nodes, then improve multiline strings
	if err := syncDoc(ctx, doc); err != nil {
		return doc, err
	}

	root := doc.GetCore().GetRootNode()
	improveMultilineStrings(ctx, root)

	// Reload from modified YAML to create fresh document that won't be overwritten by sync during marshal
	newDoc, err := reloadFromYAML(ctx, root)
	if err != nil {
		return doc, err
	}

	return newDoc, nil
}

// Trim trailing whitespace from multiline strings
// Convert any single-line strings that contain line breaks to multi-line strings
func improveMultilineStrings(ctx context.Context, node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.ScalarNode {
		if node.Style == yaml.LiteralStyle || node.Style == yaml.FoldedStyle || strings.Contains(node.Value, "\n") {
			// Trim trailing whitespace from each line in multiline strings
			// This is necessary because otherwise when re-encoded to yaml, the multiline style will be lost
			lines := strings.Split(node.Value, "\n")
			anyChanged := false
			for i, line := range lines {
				lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
				if len(line) != len(lines[i]) {
					anyChanged = true
				}
			}
			if anyChanged {
				log.From(ctx).Printf("Removed trailing whitespace within multi-line string")
				node.Value = strings.Join(lines, "\n")
			}

			// Convert single-line strings that contain line breaks to multi-line strings
			node.Style = yaml.LiteralStyle
		}
	} else {
		for _, child := range node.Content {
			improveMultilineStrings(ctx, child)
		}
	}
}
