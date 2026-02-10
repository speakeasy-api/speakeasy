package errorCodes

import (
	"fmt"

	"github.com/speakeasy-api/openapi/openapi"
	coreopenapi "github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
)

var errorGroupsDiagnose = initErrorGroups()

func Diagnose(doc *openapi.OpenAPI) suggestions.Diagnosis {
	diagnosis := suggestions.Diagnosis{}

	for op := range coreopenapi.IterateOperations(doc) {
		method, path, operation := op.Method, op.Path, op.Operation

		schemaPath := coreopenapi.GetOperationSchemaPath(path, method)

		if operation.Responses.Len() == 0 {
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
	}

	return diagnosis
}

func getMissingErrorCodes(operation *openapi.Operation) []string {
	var missingCodes []string

	for _, code := range errorGroupsDiagnose.AllCodes() {
		if responseRef, _ := getResponseForCode(&operation.Responses, code); responseRef == nil {
			missingCodes = append(missingCodes, code)
		}
	}

	return missingCodes
}
