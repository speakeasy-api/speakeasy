package schemas

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// IsSwaggerDocument checks if a document is a Swagger 2.0 specification
func IsSwaggerDocument(ctx context.Context, schemaPath string) (bool, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return false, fmt.Errorf("failed to read document: %w", err)
	}

	// Try to parse as YAML first (works for both YAML and JSON)
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// If YAML parsing fails, try JSON
		if err := json.Unmarshal(data, &doc); err != nil {
			return false, fmt.Errorf("failed to parse document: %w", err)
		}
	}

	// Check for the "swagger" field which indicates Swagger 2.0
	if swagger, ok := doc["swagger"]; ok {
		if swaggerStr, ok := swagger.(string); ok && swaggerStr == "2.0" {
			return true, nil
		}
	}

	return false, nil
}
