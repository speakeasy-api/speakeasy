package merge

import (
	"context"
	"fmt"
	"strings"

	"github.com/speakeasy-api/openapi/extensions"
	"github.com/speakeasy-api/openapi/jsonschema/oas3"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/references"
	"github.com/speakeasy-api/openapi/sequencedmap"
	"gopkg.in/yaml.v3"
)

// MergeInput represents a document to merge with optional namespace
type MergeInput struct {
	Path      string // File path to the OpenAPI document
	Namespace string // Optional namespace prefix for components/schemas
}

// MergeOptions contains options for the merge operation
type MergeOptions struct {
	DefaultRuleset         string
	WorkingDir             string
	SkipGenerateLintReport bool
	YAMLOutput             bool
}

// validateNamespaces checks that namespace usage is consistent across all inputs.
// If any input has a namespace, ALL inputs must have a namespace.
func validateNamespaces(inputs []MergeInput) error {
	hasNamespace := false
	missingNamespace := false

	for _, input := range inputs {
		if input.Namespace != "" {
			hasNamespace = true
		} else {
			missingNamespace = true
		}
	}

	if hasNamespace && missingNamespace {
		return fmt.Errorf("if namespaces are used, ALL documents must provide a namespace")
	}

	return nil
}

// validateNamespaceSlice validates namespace slice against schema count.
// If namespaces are provided, the count must match the schema count and all must be non-empty.
func validateNamespaceSlice(namespaces []string, schemaCount int) error {
	// If no namespaces provided, that's valid (no namespacing)
	if len(namespaces) == 0 {
		return nil
	}

	// If namespaces are provided, count must match
	if len(namespaces) != schemaCount {
		return fmt.Errorf("namespace count (%d) must match schema count (%d)", len(namespaces), schemaCount)
	}

	// Check for mixed usage (some empty, some non-empty)
	hasNamespace := false
	hasEmpty := false
	for _, ns := range namespaces {
		if ns != "" {
			hasNamespace = true
		} else {
			hasEmpty = true
		}
	}

	if hasNamespace && hasEmpty {
		return fmt.Errorf("if namespaces are used, ALL documents must provide a namespace")
	}

	return nil
}

// applyNamespaceToSchemas applies namespace prefixes to all schemas in components/schemas.
// It returns a mapping of old schema names to new schema names for reference updates.
// For each namespaced schema, it adds:
//   - x-speakeasy-name-override: original schema name
//   - x-speakeasy-model-namespace: namespace value
func applyNamespaceToSchemas(ctx context.Context, doc *openapi.OpenAPI, namespace string) (map[string]string, error) {
	schemaMappings := make(map[string]string)

	if namespace == "" {
		return schemaMappings, nil
	}

	if doc.Components == nil || doc.Components.Schemas == nil {
		return schemaMappings, nil
	}

	// Create a new schemas map with namespaced names
	newSchemas := sequencedmap.New[string, *oas3.JSONSchema[oas3.Referenceable]]()

	for name, schema := range doc.Components.Schemas.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		schemaMappings[name] = newName

		// Add speakeasy extensions to the schema
		if schema != nil && schema.IsSchema() {
			schemaObj := schema.GetSchema()
			if schemaObj != nil {
				// Create yaml.Node values for extensions
				nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
				namespaceNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: namespace}

				if schemaObj.Extensions == nil {
					schemaObj.Extensions = extensions.New(
						extensions.NewElem("x-speakeasy-name-override", nameNode),
						extensions.NewElem("x-speakeasy-model-namespace", namespaceNode),
					)
				} else {
					schemaObj.Extensions.Set("x-speakeasy-name-override", nameNode)
					schemaObj.Extensions.Set("x-speakeasy-model-namespace", namespaceNode)
				}
			}
		}

		newSchemas.Set(newName, schema)
	}

	// Replace the schemas map
	doc.Components.Schemas = newSchemas

	return schemaMappings, nil
}

// updateSchemaReferencesInDocument updates all $ref values pointing to schemas
// based on the provided schemaMappings.
func updateSchemaReferencesInDocument(ctx context.Context, doc *openapi.OpenAPI, schemaMappings map[string]string) error {
	if len(schemaMappings) == 0 {
		return nil
	}

	// Walk through the document and update schema references
	for item := range openapi.Walk(ctx, doc) {
		err := item.Match(openapi.Matcher{
			Schema: func(schema *oas3.JSONSchema[oas3.Referenceable]) error {
				if schema == nil {
					return nil
				}
				return updateSchemaReference(schema, schemaMappings)
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update reference at %s: %w", item.Location.ToJSONPointer().String(), err)
		}
	}

	return nil
}

// updateSchemaReference updates a single schema's $ref if it points to a renamed schema
func updateSchemaReference(schema *oas3.JSONSchema[oas3.Referenceable], schemaMappings map[string]string) error {
	if schema == nil {
		return nil
	}

	if schema.IsReference() {
		ref := schema.GetRef()
		refStr := string(ref)

		if strings.HasPrefix(refStr, "#/components/schemas/") {
			componentName := strings.TrimPrefix(refStr, "#/components/schemas/")
			if newName, exists := schemaMappings[componentName]; exists {
				newRef := "#/components/schemas/" + newName
				// Update the reference by modifying the underlying schema object's Ref field
				schemaObj := schema.GetSchema()
				if schemaObj != nil && schemaObj.Ref != nil {
					*schemaObj.Ref = references.Reference(newRef)
				}
			}
		}
	}

	return nil
}
