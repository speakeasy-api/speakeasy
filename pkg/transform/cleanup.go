package transform

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/references"
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

func Cleanup(ctx context.Context, schemaPath string, doc *openapi.OpenAPI, _ interface{}) (*openapi.OpenAPI, error) {
	var pathsToDelete []string

	for path, pi := range doc.Paths.All() {
		_, err := pi.Resolve(ctx, references.ResolveOptions{
			RootDocument:   doc,
			TargetDocument: doc,
			TargetLocation: schemaPath,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve ref %s: %w", pi.GetReference(), err)
		}

		if pi.GetObject().Len() == 0 {
			pathsToDelete = append(pathsToDelete, path)
		}
	}

	for _, path := range pathsToDelete {
		log.From(ctx).Printf("Dropped empty path: %s\n", path)
		doc.Paths.Delete(path)
	}

	// Special case where Cleanup syncs itself so it can improve multiline strings
	if err := openapi.Sync(ctx, doc); err != nil {
		return nil, err
	}
	improveMultilineStrings(ctx, doc.GetRootNode())

	return doc, nil
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
