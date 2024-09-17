package utils

import (
	"github.com/AlekSi/pointer"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
)

func ConvertOASSummary(summary openapi.Summary) shared.OASSummary {
	var operations []shared.OASOperation
	for _, operation := range summary.Operations {
		o := shared.OASOperation{
			Description: operation.Description,
			Method:      operation.Method,
			OperationID: operation.OperationID,
			Path:        operation.Path,
			Tags:        operation.Tags,
		}

		if operation.GroupOverride != "" {
			o.GroupOverride = pointer.ToString(operation.GroupOverride)
		}
		if operation.MethodNameOverride != "" {
			o.MethodNameOverride = pointer.ToString(operation.MethodNameOverride)
		}

		operations = append(operations, o)
	}

	return shared.OASSummary{
		Info: shared.OASInfo{
			Title:       summary.Info.Title,
			Description: summary.Info.Description,
		},
		Operations: operations,
	}

}

func ConvertDiagnosis(diagnosis suggestions.Diagnosis) []shared.Diagnostic {
	var diagnostics []shared.Diagnostic
	for t, ds := range diagnosis {
		for _, d := range ds {
			diagnostics = append(diagnostics, shared.Diagnostic{
				Message: d.Message,
				Path:    d.SchemaPath,
				Type:    string(t),
			})
		}
	}

	return diagnostics
}
