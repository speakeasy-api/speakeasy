package transform

import (
	"bytes"
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"io"
	"slices"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func FilterOperations(ctx context.Context, schemaPath string, includeOps []string, include bool, w io.Writer) error {
	return transformer[args]{
		schemaPath:  schemaPath,
		transformFn: filterOperations,
		w:           w,
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

func filterOperations(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], args args) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	overlay := BuildFilterOperationsOverlay(model, args.include, args.includeOps, nil)

	root := model.Index.GetRootNode()
	if err := overlay.ApplyTo(root); err != nil {
		return doc, model, err
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

func BuildRemoveInvalidOperationsOverlay(model *libopenapi.DocumentModel[v3.Document], opToErr map[string]error) overlay.Overlay {
	return BuildFilterOperationsOverlay(model, false, maps.Keys(opToErr), opToErr)
}

func BuildFilterOperationsOverlay(model *libopenapi.DocumentModel[v3.Document], include bool, ops []string, opToErr map[string]error) overlay.Overlay {
	actionFn := func(method, path string, operation *v3.Operation) (map[string]string, *overlay.Action, *suggestions.ModificationExtension) {
		operationID := operation.OperationId

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
