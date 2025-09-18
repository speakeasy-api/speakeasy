package transform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"gopkg.in/yaml.v3"
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

func filterOperations(ctx context.Context, schemaPath string, doc *openapi.OpenAPI, args args) (*openapi.OpenAPI, error) {
	overlay := BuildFilterOperationsOverlay(doc, args.include, args.includeOps, nil)

	if err := overlay.ApplyTo(doc.GetRootNode()); err != nil {
		return doc, err
	}

	newSpec := bytes.Buffer{}
	enc := yaml.NewEncoder(&newSpec)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return doc, model, err
	}

	// Unfortunately, in order to remove orphans, we need to re-render the document.
	updatedDoc, _, err := openapi.Load(newSpec.Bytes(), args.schemaPath)
	if err != nil {
		return doc, model, err
	}
	doc = *updatedDoc

	_, model, err = RemoveOrphans(ctx, doc, nil, nil)
	if err != nil {
		return doc, model, err
	}

	_, model, err = Cleanup(ctx, doc, model, nil)
	if err != nil {
		return doc, model, err
	}

	return doc, model, err
}

func BuildRemoveInvalidOperationsOverlay(doc *openapi.OpenAPI, opToErr map[string]error) overlay.Overlay {
	return BuildFilterOperationsOverlay(doc, false, slices.Collect(maps.Keys(opToErr)), opToErr)
}

func BuildFilterOperationsOverlay(doc *openapi.OpenAPI, include bool, ops []string, opToErr map[string]error) overlay.Overlay {
	actionFn := func(method, path string, operation *openapi.Operation) (map[string]string, *overlay.Action, *suggestions.ModificationExtension) {
		operationID := operation.GetOperationID()

		if operationID == "" {
			operationID = fmt.Sprintf("%s_%s", method, path)
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
			target := overlay.NewTargetSelector(path, method)
			action := overlay.Action{
				Target: target,
				Remove: true,
			}

			before := "<invalid_operation>"
			if err, ok := opToErr[operationID]; ok {
				before = err.Error()
			}

			return nil, &action, &suggestions.ModificationExtension{
				Type:   suggestions.ModificationTypeRemoveInvalid,
				Before: before,
				After:  "<removed>",
			}
		}

		return nil, nil, nil
	}

	builder := suggestions.ModificationsBuilder{
		ActionFn: actionFn,
	}

	return builder.Construct(model.Model)
}
