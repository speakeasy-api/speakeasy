package transform

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/speakeasy-api/openapi/marshaller"
	"github.com/speakeasy-api/openapi/openapi"
)

// Snip removes specified operations from an OpenAPI document and cleans up unused components.
//
// Operations can be specified by:
//   - Operation ID: "getUserById"
//   - Path and method: "/users/{id}:GET"
//
// The function supports two modes:
//   - Remove mode (keepMode=false): Removes the specified operations
//   - Keep mode (keepMode=true): Keeps only the specified operations, removes everything else
func Snip(ctx context.Context, schemaPath string, operations []string, keepMode bool, w io.Writer) error {
	// Open and read the input file
	f, err := os.Open(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to open schema file: %w", err)
	}
	defer f.Close()

	// Parse the OpenAPI document using the openapi package
	doc, validationErrors, err := openapi.Unmarshal(ctx, f)
	if err != nil {
		return fmt.Errorf("failed to unmarshal OpenAPI document: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("failed to parse OpenAPI document: document is nil")
	}

	// Report validation errors to stderr (if any)
	if len(validationErrors) > 0 {
		fmt.Fprintf(os.Stderr, "⚠️  Found %d validation error(s) in the document:\n", len(validationErrors))
		for i, validationErr := range validationErrors {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, validationErr.Error())
		}
		fmt.Fprintln(os.Stderr)
	}

	// Parse operation identifiers from the input strings
	operationsToProcess, err := parseOperationIdentifiers(operations)
	if err != nil {
		return err
	}

	if len(operationsToProcess) == 0 {
		return fmt.Errorf("no operations specified")
	}

	// If in keep mode, we need to invert the operation list
	// (find all operations, then remove the ones not in the keep list)
	if keepMode {
		operationsToProcess, err = invertOperationList(ctx, doc, operationsToProcess)
		if err != nil {
			return err
		}
	}

	// Perform the snip operation (this also runs Clean() automatically)
	removed, err := openapi.Snip(ctx, doc, operationsToProcess)
	if err != nil {
		return fmt.Errorf("failed to snip operations: %w", err)
	}

	// Write success message to stderr so stdout can be piped
	if keepMode {
		fmt.Fprintf(os.Stderr, "✅ Kept %d operation(s), removed %d operation(s)\n", len(operations), removed)
	} else {
		fmt.Fprintf(os.Stderr, "✅ Removed %d operation(s) and cleaned unused components\n", removed)
	}

	// Marshal the modified document to the output writer
	if err := marshaller.Marshal(ctx, doc, w); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// parseOperationIdentifiers converts string representations into OperationIdentifier structs.
// Supports two formats:
//   - Operation ID: "getUserById"
//   - Path and method: "/users/{id}:GET"
func parseOperationIdentifiers(operations []string) ([]openapi.OperationIdentifier, error) {
	var identifiers []openapi.OperationIdentifier

	for _, op := range operations {
		if op == "" {
			continue
		}

		// Check if it's in path:method format
		if strings.Contains(op, ":") {
			parts := strings.SplitN(op, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid operation format: %s (expected path:METHOD or operationId)", op)
			}

			path := parts[0]
			method := strings.ToUpper(parts[1])

			if path == "" || method == "" {
				return nil, fmt.Errorf("invalid operation format: %s (path and method cannot be empty)", op)
			}

			identifiers = append(identifiers, openapi.OperationIdentifier{
				Path:   path,
				Method: method,
			})
		} else {
			// Treat as operation ID
			identifiers = append(identifiers, openapi.OperationIdentifier{
				OperationID: op,
			})
		}
	}

	return identifiers, nil
}

// invertOperationList converts a "keep" list into a "remove" list by collecting
// all operations in the document and removing the ones in the keep list.
func invertOperationList(ctx context.Context, doc *openapi.OpenAPI, keepList []openapi.OperationIdentifier) ([]openapi.OperationIdentifier, error) {
	if doc.Paths == nil || doc.Paths.Len() == 0 {
		return nil, fmt.Errorf("no operations found in document")
	}

	// Build lookup sets for the keep list
	keepByID := make(map[string]bool)
	keepByPathMethod := make(map[string]bool)

	for _, keep := range keepList {
		if keep.OperationID != "" {
			keepByID[keep.OperationID] = true
		}
		if keep.Path != "" && keep.Method != "" {
			key := strings.ToUpper(keep.Method) + " " + keep.Path
			keepByPathMethod[key] = true
		}
	}

	// Collect all operations and determine which ones to remove
	var operationsToRemove []openapi.OperationIdentifier

	for path, pathItem := range doc.Paths.All() {
		if pathItem == nil || pathItem.Object == nil {
			continue
		}

		for method, operation := range pathItem.Object.All() {
			if operation == nil {
				continue
			}

			// Check if this operation is in the keep list
			shouldKeep := false

			// Check by operation ID
			opID := operation.GetOperationID()
			if opID != "" && keepByID[opID] {
				shouldKeep = true
			}

			// Check by path and method
			key := strings.ToUpper(string(method)) + " " + path
			if keepByPathMethod[key] {
				shouldKeep = true
			}

			// If not in keep list, add to remove list
			if !shouldKeep {
				operationsToRemove = append(operationsToRemove, openapi.OperationIdentifier{
					Path:   path,
					Method: strings.ToUpper(string(method)),
				})
			}
		}
	}

	if len(operationsToRemove) == 0 {
		return nil, fmt.Errorf("all operations would be kept; nothing to remove")
	}

	return operationsToRemove, nil
}
