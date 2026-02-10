package transform

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/overlay"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
)

func FilterOperations(ctx context.Context, schemaPath string, includeOps []string, include bool, yamlOut bool, w io.Writer) error {
	return transformer[args]{
		schemaPath:  schemaPath,
		transformFn: filterOperations,
		w:           w,
		jsonOut:     !yamlOut,
		args: args{
			includeOps: includeOps,
			include:    include,
			schemaPath: schemaPath,
		},
	}.Do(ctx)
}

func FilterOperationsFromReader(ctx context.Context, schema io.Reader, schemaPath string, includeOps []string, include bool, w io.Writer, yamlOut bool) error {
	return transformer[args]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: filterOperations,
		w:           w,
		jsonOut:     !yamlOut,
		args: args{
			includeOps: includeOps,
			include:    include,
			schemaPath: schemaPath,
		},
	}.Do(ctx)
}

type args struct {
	includeOps []string
	include    bool
	schemaPath string
}

func filterOperations(ctx context.Context, doc *openapi.OpenAPI, args args) (*openapi.OpenAPI, error) {
	ol := BuildFilterOperationsOverlay(doc, args.include, args.includeOps, nil)

	// Sync to get current YAML nodes
	if err := syncDoc(ctx, doc); err != nil {
		return doc, err
	}

	root := doc.GetCore().GetRootNode()
	if err := ol.ApplyTo(root); err != nil {
		return doc, err
	}

	// Reload from modified YAML
	doc, err := reloadFromYAML(ctx, root)
	if err != nil {
		return doc, err
	}

	// Remove orphaned components
	doc, err = RemoveOrphans(ctx, doc, nil)
	if err != nil {
		return doc, err
	}

	// Clean up empty paths
	doc, err = Cleanup(ctx, doc, nil)
	if err != nil {
		return doc, err
	}

	return doc, nil
}

func BuildRemoveInvalidOperationsOverlay(doc *openapi.OpenAPI, opToErr map[string]error) overlay.Overlay {
	return BuildFilterOperationsOverlay(doc, false, slices.Collect(maps.Keys(opToErr)), opToErr)
}

func BuildFilterOperationsOverlay(doc *openapi.OpenAPI, include bool, ops []string, opToErr map[string]error) overlay.Overlay {
	var actions []overlay.Action

	if doc.Paths == nil {
		return overlay.Overlay{Actions: actions}
	}

	for path, pathItem := range doc.Paths.All() {
		if pathItem == nil || pathItem.Object == nil {
			continue
		}

		for method, operation := range pathItem.Object.All() {
			if operation == nil {
				continue
			}

			operationID := ""
			if operation.OperationID != nil {
				operationID = *operation.OperationID
			}
			if operationID == "" {
				operationID = fmt.Sprintf("%s_%s", string(method), path)
			}

			remove := false
			if include {
				if !slices.Contains(ops, operationID) {
					remove = true
				}
			} else {
				if slices.Contains(ops, operationID) {
					remove = true
				}
			}

			if remove {
				target := overlay.NewTargetSelector(path, string(method))
				action := overlay.Action{
					Target: target,
					Remove: true,
				}

				before := "<invalid_operation>"
				if err, ok := opToErr[operationID]; ok {
					before = err.Error()
				}

				suggestions.AddModificationExtension(&action, &suggestions.ModificationExtension{
					Type:   suggestions.ModificationTypeRemoveInvalid,
					Before: before,
					After:  "<removed>",
				})

				actions = append(actions, action)
			}
		}
	}

	return overlay.Overlay{Actions: actions}
}
