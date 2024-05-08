package transform

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

type filterOpsArgs struct {
	ops     []string
	include bool
}

func FilterOperations(ctx context.Context, schemaPath string, includeOps []string, include bool, w io.Writer) error {
	return transformer[filterOpsArgs]{
		schemaPath:  schemaPath,
		transformFn: filterOperations,
		w:           w,
		args:        filterOpsArgs{ops: includeOps, include: include},
	}.Do(ctx)
}

func filterOperations(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], args filterOpsArgs) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	pathItems := model.Model.Paths.PathItems
	var pathsToDelete []string

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
			if args.include {
				if !slices.Contains(args.ops, operationID) {
					toDelete = append(toDelete, method)
				}
			} else {
				if slices.Contains(args.ops, operationID) {
					toDelete = append(toDelete, method)
				}
			}
		}

		for _, method := range toDelete {
			operations.Delete(method)
			deleteOperation(pathVal, method)
		}

		if operations.Len() == 0 {
			pathsToDelete = append(pathsToDelete, path)
		}
	}

	for _, path := range pathsToDelete {
		pathItems.Delete(path)
	}

	// Do some extra cleanup to remove anything now orphaned
	return RemoveOrphans(ctx, doc, model, nil)
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
