package transform

import (
	"context"
	"io"
	"strings"
	"unicode"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/speakeasy-core/openapi"
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

func Cleanup(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], _ interface{}) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	pathItems := model.Model.Paths.PathItems
	var pathsToDelete []string

	for pathPair := orderedmap.First(pathItems); pathPair != nil; pathPair = pathPair.Next() {
		path := pathPair.Key()
		pathVal := pathPair.Value()
		operations := pathVal.GetOperations()
		if operations.Len() == 0 {
			pathsToDelete = append(pathsToDelete, path)
		}
	}

	for _, path := range pathsToDelete {
		log.From(ctx).Printf("Dropped empty path: %s\n", path)
		pathItems.Delete(path)
	}

	// Unfortunately, rendering and reloading is the only way to "apply" the path changes
	_, model, err := reload(model, doc.GetConfiguration().BasePath)
	if err != nil {
		return doc, model, err
	}

	root := model.Index.GetRootNode()
	improveMultilineStrings(ctx, root)

	// Render and reload the document to ensure that the changes are reflected in the model
	updatedDoc, err := yaml.Marshal(root)
	if err != nil {
		return doc, model, err
	}

	docNew, model, err := openapi.Load(updatedDoc, doc.GetConfiguration().BasePath)

	if err != nil {
		return doc, model, err
	}

	return *docNew, model, nil
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
