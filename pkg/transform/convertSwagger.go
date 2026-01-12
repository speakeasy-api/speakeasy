package transform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/swagger"
)

// ConvertSwagger upgrades a Swagger 2.0 document to OpenAPI 3.0 using the speakeasy-api/openapi library
func ConvertSwagger(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read swagger document: %w", err)
	}

	return ConvertSwaggerFromReader(ctx, bytes.NewReader(data), schemaPath, w, yamlOut)
}

// ConvertSwaggerFromReader upgrades a Swagger 2.0 document to OpenAPI 3.0 from an io.Reader
func ConvertSwaggerFromReader(ctx context.Context, r io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	// Read all data from the reader
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read swagger document: %w", err)
	}

	// Unmarshal using swagger.Unmarshal (returns swagger doc, validation errors, and error)
	swaggerDoc, validationErrs, err := swagger.Unmarshal(ctx, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to unmarshal swagger document: %w", err)
	}

	// Log validation errors but continue (they're warnings)
	if len(validationErrs) > 0 {
		// Validation errors are non-fatal, just warnings
		_, _ = fmt.Fprintf(w, "# Swagger document has %d validation warnings\n", len(validationErrs))
	}

	// Upgrade to OpenAPI 3.0
	openapi3Doc, err := swagger.Upgrade(ctx, swaggerDoc)
	if err != nil {
		return fmt.Errorf("failed to upgrade swagger to openapi 3.0: %w", err)
	}

	// Marshal the OpenAPI 3.0 document
	if err := openapi.Marshal(ctx, openapi3Doc, w); err != nil {
		return fmt.Errorf("failed to marshal openapi document: %w", err)
	}

	return nil
}
