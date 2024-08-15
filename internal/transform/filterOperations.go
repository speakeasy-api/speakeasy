package transform

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"gopkg.in/yaml.v3"
	"io"
	"slices"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func FilterOperations(ctx context.Context, schemaPath string, includeOps []string, include bool, w io.Writer) error {
	_, _, model, err := openapi.LoadDocument(ctx, schemaPath)
	if err != nil {
		return err
	}

	overlay := BuildFilterOperationsOverlay(model, include, includeOps)

	root := model.Index.GetRootNode()
	if err := overlay.ApplyTo(root); err != nil {
		return err
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(root)
}

func BuildFilterOperationsOverlay(model *libopenapi.DocumentModel[v3.Document], include bool, ops []string) overlay.Overlay {
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
			return nil, &action, &suggestions.ModificationExtension{
				Type:   suggestions.ModificationTypeRemoveInvalid,
				Before: "<invalid_operation>", // TODO: insert the actual error message here
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
