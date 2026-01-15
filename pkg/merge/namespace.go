package merge

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/speakeasy-api/openapi/extensions"
	"github.com/speakeasy-api/openapi/jsonpointer"
	"github.com/speakeasy-api/openapi/jsonschema/oas3"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/references"
	"github.com/speakeasy-api/openapi/sequencedmap"
	"gopkg.in/yaml.v3"
)

const schemasRefPrefix = "#/components/schemas/"

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

// validNamespacePattern defines the allowed characters for namespace values.
// Namespaces must start with a letter and contain only alphanumeric characters and underscores.
var validNamespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

// validateNamespace checks that a namespace value contains only valid characters.
// Valid namespaces must start with a letter and contain only alphanumeric characters and underscores.
func validateNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}

	if !validNamespacePattern.MatchString(namespace) {
		return fmt.Errorf("invalid namespace %q: must contain only alphanumeric characters (a-z, A-Z, 0-9), "+
			"underscores (_), hyphens (-), and dots (.). "+
			"Nested namespaces using forward slashes (/) are not currently supported", namespace)
	}

	return nil
}

// validateNamespaces validates all namespace values in the inputs.
// Empty namespaces are allowed and will result in no prefixing for that document.
func validateNamespaces(inputs []MergeInput) error {
	for _, input := range inputs {
		if err := validateNamespace(input.Namespace); err != nil {
			return fmt.Errorf("input %s: %w", input.Path, err)
		}
	}

	return nil
}

// validateNamespaceSlice validates namespace slice against schema count.
// Empty namespaces are allowed and will result in no prefixing for that document.
func validateNamespaceSlice(namespaces []string, schemaCount int) error {
	// If no namespaces provided, that's valid (no namespacing)
	if len(namespaces) == 0 {
		return nil
	}

	// If namespaces are provided, count must match
	if len(namespaces) != schemaCount {
		return fmt.Errorf("namespace count (%d) must match schema count (%d)", len(namespaces), schemaCount)
	}

	// Validate each namespace value
	for i, ns := range namespaces {
		if err := validateNamespace(ns); err != nil {
			return fmt.Errorf("namespace at index %d: %w", i, err)
		}
	}

	return nil
}

// applyNamespaceToSchemas applies namespace prefixes to all schemas in components/schemas.
// For each namespaced schema, it adds:
//   - x-speakeasy-name-override: original schema name
//   - x-speakeasy-model-namespace: namespace value
//
// The function mutates doc.Components.Schemas in place. Reference updates should be
// performed by calling updateSchemaReferencesInDocument after this function.
func applyNamespaceToSchemas(doc *openapi.OpenAPI, namespace string) {
	if namespace == "" {
		return
	}

	if doc.Components == nil || doc.Components.Schemas == nil {
		return
	}

	// Create a new schemas map with namespaced names
	newSchemas := sequencedmap.New[string, *oas3.JSONSchema[oas3.Referenceable]]()

	for name, schema := range doc.Components.Schemas.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)

		// Add speakeasy extensions to the schema.
		// Note: We still rename the schema even if we can't add extensions (e.g., for pure references).
		// The extensions are used by the SDK generator for model organization but aren't strictly required
		// for the merge to function correctly.
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
}

// updateSchemaReferencesInDocument updates all $ref values pointing to schemas
// based on the namespace. It determines the mapping from the x-speakeasy-name-override
// extension in the document's schemas.
func updateSchemaReferencesInDocument(ctx context.Context, doc *openapi.OpenAPI, namespace string) error {
	if namespace == "" {
		return nil
	}

	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil
	}

	// Build schema mappings from the document's schemas using the x-speakeasy-name-override extension
	schemaMappings := make(map[string]string)
	for newName, schema := range doc.Components.Schemas.All() {
		if schema != nil && schema.IsSchema() {
			schemaObj := schema.GetSchema()
			if schemaObj != nil && schemaObj.Extensions != nil {
				if nameOverrideNode, ok := schemaObj.Extensions.Get("x-speakeasy-name-override"); ok {
					var originalName string
					if err := nameOverrideNode.Decode(&originalName); err == nil && originalName != "" {
						schemaMappings[originalName] = newName
					}
				}
			}
		}
	}

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

// updateSchemaReference updates a single schema's $ref if it points to a renamed schema.
// Supports both direct schema references (e.g., #/components/schemas/Pet) and property references
// (e.g., #/components/schemas/Pet/properties/name).
func updateSchemaReference(schema *oas3.JSONSchema[oas3.Referenceable], schemaMappings map[string]string) error {
	if schema == nil {
		return nil
	}

	if !schema.IsReference() {
		return nil
	}

	ref := schema.GetRef()

	// Validate the reference format using the references package
	if err := ref.Validate(); err != nil {
		return fmt.Errorf("invalid reference %q: %w", ref, err)
	}

	// Check if this is a local reference with a JSON pointer
	if !ref.HasJSONPointer() {
		return nil
	}

	refStr := string(ref)

	// Only process schema references
	if !strings.HasPrefix(refStr, schemasRefPrefix) {
		return nil
	}

	// Use JSON pointer to parse the reference path
	jp := ref.GetJSONPointer()
	jpStr := jp.String()

	// Parse the JSON pointer into parts (e.g., "/components/schemas/Pet" -> ["components", "schemas", "Pet"])
	// or for property refs: "/components/schemas/Pet/properties/name" -> ["components", "schemas", "Pet", "properties", "name"]
	parts := strings.Split(strings.TrimPrefix(jpStr, "/"), "/")

	// Need at least 3 parts for a valid schema reference: ["components", "schemas", "<schemaName>"]
	if len(parts) < 3 || parts[0] != "components" || parts[1] != "schemas" {
		return nil
	}

	componentName := parts[2]
	if newName, exists := schemaMappings[componentName]; exists {
		// Update the schema name part and preserve any additional path segments (for property references)
		parts[2] = newName
		newPointer := jsonpointer.PartsToJSONPointer(parts)
		newRef := references.Reference("#" + newPointer.String())

		// Update the reference by modifying the underlying schema object's Ref field
		schemaObj := schema.GetSchema()
		if schemaObj != nil && schemaObj.Ref != nil {
			*schemaObj.Ref = newRef
		}
	}

	return nil
}
