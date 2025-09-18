package errorCodes

import (
	"context"
	"fmt"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
)

var errorGroupsDiagnose = initErrorGroups()

func Diagnose(ctx context.Context, document *openapi.OpenAPI) suggestions.Diagnosis {
	diagnosis := suggestions.Diagnosis{}

	for item := range openapi.Walk(ctx, document) {
		_ = item.Match(openapi.Matcher{
			Operation: func(operation *openapi.Operation) error {
				method, path := openapi.ExtractMethodAndPath(item.Location)

				schemaPath := getOperationSchemaPath(path, method)

				if operation.GetResponses().Len() == 0 {
					diagnosis.Add(suggestions.MissingErrorCodes, suggestions.Diagnostic{
						SchemaPath: schemaPath,
						Message:    fmt.Sprintf("Operation %s (%s %s) has no responses defined!", operation.GetOperationID(), method, path),
					})
				}

				missingCodes := getMissingErrorCodes(operation)
				if len(missingCodes) > 0 {
					diagnosis.Add(suggestions.MissingErrorCodes, suggestions.Diagnostic{
						SchemaPath: schemaPath,
						Message:    fmt.Sprintf("Operation %s (%s %s) is missing definitions for %d recommended error codes", operation.GetOperationID(), method, path, len(missingCodes)),
					})
				}

				return nil
			},
		})
	}

	return diagnosis
}

func getMissingErrorCodes(operation *openapi.Operation) []string {
	var missingCodes []string

	for _, code := range errorGroupsDiagnose.AllCodes() {
		if response, _ := getResponseForCode(operation.GetResponses(), code); response == nil {
			missingCodes = append(missingCodes, code)
		}
	}

	return missingCodes
}

func getOperationSchemaPath(path, method string) []string {
	return []string{"paths", path, method}
}
