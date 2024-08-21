package transform

import (
	"context"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"io"
)

func CleanupDocument(ctx context.Context, schemaPath string, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: Cleanup,
		w:           w,
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
		log.From(ctx).Printf("dropped %s\n", path)
		pathItems.Delete(path)
	}

	return doc, model, nil
}
