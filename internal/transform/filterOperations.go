package transform

import (
	"context"
	"fmt"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"io"
	"slices"
)

func FilterOperations(ctx context.Context, schemaPath string, operationIDs []string, w io.Writer) error {
	return transformer[[]string]{
		schemaPath:  schemaPath,
		transformFn: filterOperations,
		w:           w,
		args:        operationIDs,
	}.Do(ctx)

}

func filterOperations(doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], operationIDs []string) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	pathItems := model.Model.Paths.PathItems
	for pathPair := orderedmap.First(pathItems); pathPair != nil; pathPair = pathPair.Next() {
		path := pathPair.Key()
		pathVal := pathPair.Value()
		operations := pathVal.GetOperations()
		var toDelete []string

		for operationPair := orderedmap.First(operations); operationPair != nil; operationPair = operationPair.Next() {
			method := operationPair.Key()
			operation := operationPair.Value()
			operationID := operation.OperationId
			if operationID == "" {
				operationID = fmt.Sprintf("%s_%s", method, path)
			}
			if !slices.Contains(operationIDs, operationID) {
				toDelete = append(toDelete, method)
			}
		}

		for _, method := range toDelete {
			operations.Delete(method)
			deleteOperation(pathVal, method)
		}

		if operations.Len() == 0 {
			pathItems.Delete(path)
		}
	}

	// Do some extra cleanup to remove anything now orphaned
	return RemoveOrphans(doc, model, nil)
}

func deleteOperation(pathVal *v3.PathItem, method string) {
	switch method {
	case "get":
		pathVal.Get = nil
	case "put":
		pathVal.Put = nil
	case "post":
		pathVal.Post = nil
	case "delete":
		pathVal.Delete = nil
	case "options":
		pathVal.Options = nil
	case "head":
		pathVal.Head = nil
	case "patch":
		pathVal.Patch = nil
	case "trace":
		pathVal.Trace = nil
	}
}
