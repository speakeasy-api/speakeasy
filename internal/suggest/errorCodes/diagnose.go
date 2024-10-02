package errorCodes

import (
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
)

var errorGroupsDiagnose = initErrorGroups()

func Diagnose(document v3.Document) suggestions.Diagnosis {
	diagnosis := suggestions.Diagnosis{}

	for op := range openapi.IterateOperations(document) {
		method, path, operation := op.Method, op.Path, op.Operation

		schemaPath := openapi.GetOperationSchemaPath(path, method)

		codes := operation.Responses.Codes
		if codes == nil {
			diagnosis.Add(suggestions.MissingErrorCodes, suggestions.Diagnostic{
				SchemaPath: schemaPath,
				Message:    fmt.Sprintf("Operation %s (%s %s) has no responses defined!", operation.OperationId, method, path),
			})
		}

		missingCodes := getMissingErrorCodes(operation)
		if len(missingCodes) > 0 {
			diagnosis.Add(suggestions.MissingErrorCodes, suggestions.Diagnostic{
				SchemaPath: schemaPath,
				Message:    fmt.Sprintf("Operation %s (%s %s) is missing definitions for %d recommended error codes", operation.OperationId, method, path, len(missingCodes)),
			})
		}
	}

	return diagnosis
}

func getMissingErrorCodes(operation *v3.Operation) []string {
	var missingCodes []string

	for _, code := range errorGroupsDiagnose.AllCodes() {
		if response, _ := getResponseForCode(operation.Responses.Codes, code); response == nil {
			missingCodes = append(missingCodes, code)
		}
	}

	return missingCodes
}
