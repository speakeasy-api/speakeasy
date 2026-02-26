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

const (
	schemasRefPrefix         = "#/components/schemas/"
	parametersRefPrefix      = "#/components/parameters/"
	responsesRefPrefix       = "#/components/responses/"
	requestBodiesRefPrefix   = "#/components/requestBodies/"
	headersRefPrefix         = "#/components/headers/"
	securitySchemesRefPrefix = "#/components/securitySchemes/"
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

// applySchemaExtensions sets x-speakeasy-name-override and x-speakeasy-model-namespace
// extensions on a schema object. This is used by all applyNamespaceTo* functions to mark
// schemas with their original name and namespace for the SDK generator.
func applySchemaExtensions(schema *oas3.JSONSchema[oas3.Referenceable], originalName, namespace string) {
	if schema == nil || !schema.IsSchema() || schema.IsReference() {
		return
	}
	schemaObj := schema.GetSchema()
	if schemaObj == nil {
		return
	}

	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: originalName}
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

// setExtensions sets x-speakeasy-name-override and x-speakeasy-model-namespace
// directly on an extensions object. Used for component types that don't have inner
// schemas (e.g., security schemes).
func setExtensions(exts **extensions.Extensions, originalName, namespace string) {
	nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: originalName}
	namespaceNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: namespace}

	if *exts == nil {
		*exts = extensions.New(
			extensions.NewElem("x-speakeasy-name-override", nameNode),
			extensions.NewElem("x-speakeasy-model-namespace", namespaceNode),
		)
	} else {
		(*exts).Set("x-speakeasy-name-override", nameNode)
		(*exts).Set("x-speakeasy-model-namespace", namespaceNode)
	}
}

// applyNamespaceToSchemas applies namespace prefixes to all schemas in components/schemas.
// For each namespaced schema, it adds:
//   - x-speakeasy-name-override: original schema name
//   - x-speakeasy-model-namespace: namespace value
//
// Returns a mapping of originalName -> newName for use in reference updates.
// Note: extensions are only applied to inline schemas, not pure $ref schemas, to avoid
// invalid YAML output. The returned mapping handles ref updates for all schemas regardless.
func applyNamespaceToSchemas(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.Schemas == nil {
		return mappings
	}

	newSchemas := sequencedmap.New[string, *oas3.JSONSchema[oas3.Referenceable]]()

	for name, schema := range doc.Components.Schemas.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName
		applySchemaExtensions(schema, name, namespace)
		newSchemas.Set(newName, schema)
	}

	doc.Components.Schemas = newSchemas
	return mappings
}

// applyNamespaceToParameters applies namespace prefixes to all parameters in components/parameters.
// For each parameter, it adds x-speakeasy-name-override and x-speakeasy-model-namespace to the
// parameter's top-level schema. Returns a mapping of originalName -> newName.
func applyNamespaceToParameters(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.Parameters == nil {
		return mappings
	}

	newParams := sequencedmap.New[string, *openapi.ReferencedParameter]()

	for name, param := range doc.Components.Parameters.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName

		if param != nil && !param.IsReference() && param.Object != nil {
			applySchemaExtensions(param.Object.Schema, name, namespace)
		}

		newParams.Set(newName, param)
	}

	doc.Components.Parameters = newParams
	return mappings
}

// applyNamespaceToResponses applies namespace prefixes to all responses in components/responses.
// For each response, it adds x-speakeasy-name-override and x-speakeasy-model-namespace to
// the schemas found in the response's content media types. Returns a mapping of originalName -> newName.
func applyNamespaceToResponses(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.Responses == nil {
		return mappings
	}

	newResponses := sequencedmap.New[string, *openapi.ReferencedResponse]()

	for name, response := range doc.Components.Responses.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName

		if response != nil && !response.IsReference() && response.Object != nil && response.Object.Content != nil {
			for _, mediaType := range response.Object.Content.All() {
				if mediaType != nil {
					applySchemaExtensions(mediaType.Schema, name, namespace)
				}
			}
		}

		newResponses.Set(newName, response)
	}

	doc.Components.Responses = newResponses
	return mappings
}

// applyNamespaceToRequestBodies applies namespace prefixes to all request bodies in components/requestBodies.
// For each request body, it adds x-speakeasy-name-override and x-speakeasy-model-namespace to
// the schemas found in the request body's content media types. Returns a mapping of originalName -> newName.
func applyNamespaceToRequestBodies(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.RequestBodies == nil {
		return mappings
	}

	newRequestBodies := sequencedmap.New[string, *openapi.ReferencedRequestBody]()

	for name, requestBody := range doc.Components.RequestBodies.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName

		if requestBody != nil && !requestBody.IsReference() && requestBody.Object != nil && requestBody.Object.Content != nil {
			for _, mediaType := range requestBody.Object.Content.All() {
				if mediaType != nil {
					applySchemaExtensions(mediaType.Schema, name, namespace)
				}
			}
		}

		newRequestBodies.Set(newName, requestBody)
	}

	doc.Components.RequestBodies = newRequestBodies
	return mappings
}

// applyNamespaceToHeaders applies namespace prefixes to all headers in components/headers.
// For each header, it adds x-speakeasy-name-override and x-speakeasy-model-namespace to the
// header's top-level schema. Returns a mapping of originalName -> newName.
func applyNamespaceToHeaders(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.Headers == nil {
		return mappings
	}

	newHeaders := sequencedmap.New[string, *openapi.ReferencedHeader]()

	for name, header := range doc.Components.Headers.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName

		if header != nil && !header.IsReference() && header.Object != nil {
			applySchemaExtensions(header.Object.Schema, name, namespace)
		}

		newHeaders.Set(newName, header)
	}

	doc.Components.Headers = newHeaders
	return mappings
}

// applyNamespaceToSecuritySchemes applies namespace prefixes to all security schemes in
// components/securitySchemes. Extensions are applied directly on the SecurityScheme object
// since security schemes don't contain inner schemas. Returns a mapping of originalName -> newName.
func applyNamespaceToSecuritySchemes(doc *openapi.OpenAPI, namespace string) map[string]string {
	mappings := make(map[string]string)
	if namespace == "" {
		return mappings
	}

	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		return mappings
	}

	newSchemes := sequencedmap.New[string, *openapi.ReferencedSecurityScheme]()

	for name, scheme := range doc.Components.SecuritySchemes.All() {
		newName := fmt.Sprintf("%s_%s", namespace, name)
		mappings[name] = newName

		if scheme != nil && !scheme.IsReference() && scheme.Object != nil {
			setExtensions(&scheme.Object.Extensions, name, namespace)
		}

		newSchemes.Set(newName, scheme)
	}

	doc.Components.SecuritySchemes = newSchemes
	return mappings
}

// namespaceMappings holds the old->new name mappings for all component types.
type namespaceMappings struct {
	Parameters      map[string]string
	Responses       map[string]string
	RequestBodies   map[string]string
	Headers         map[string]string
	SecuritySchemes map[string]string
}

// updateComponentReferencesInDocument updates all $ref values pointing to parameters,
// responses, requestBodies, headers, and securitySchemes based on the provided mappings.
// It performs a single walk pass over the document for efficiency.
func updateComponentReferencesInDocument(ctx context.Context, doc *openapi.OpenAPI, mappings namespaceMappings) error {
	if doc == nil {
		return nil
	}

	hasAnyMappings := len(mappings.Parameters) > 0 ||
		len(mappings.Responses) > 0 ||
		len(mappings.RequestBodies) > 0 ||
		len(mappings.Headers) > 0 ||
		len(mappings.SecuritySchemes) > 0

	if !hasAnyMappings {
		return nil
	}

	for item := range openapi.Walk(ctx, doc) {
		err := item.Match(openapi.Matcher{
			ReferencedParameter: func(param *openapi.ReferencedParameter) error {
				if param == nil || param.Reference == nil {
					return nil
				}
				return updateComponentRef(param.Reference, mappings.Parameters, "parameters")
			},
			ReferencedResponse: func(resp *openapi.ReferencedResponse) error {
				if resp == nil || resp.Reference == nil {
					return nil
				}
				return updateComponentRef(resp.Reference, mappings.Responses, "responses")
			},
			ReferencedRequestBody: func(rb *openapi.ReferencedRequestBody) error {
				if rb == nil || rb.Reference == nil {
					return nil
				}
				return updateComponentRef(rb.Reference, mappings.RequestBodies, "requestBodies")
			},
			ReferencedHeader: func(h *openapi.ReferencedHeader) error {
				if h == nil || h.Reference == nil {
					return nil
				}
				return updateComponentRef(h.Reference, mappings.Headers, "headers")
			},
			ReferencedSecurityScheme: func(ss *openapi.ReferencedSecurityScheme) error {
				if ss == nil || ss.Reference == nil {
					return nil
				}
				return updateComponentRef(ss.Reference, mappings.SecuritySchemes, "securitySchemes")
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update component reference at %s: %w", item.Location.ToJSONPointer().String(), err)
		}
	}

	return nil
}

// updateComponentRef updates a single Reference pointer if it matches a component mapping.
// componentType should be one of: "parameters", "responses", "requestBodies", "headers", "securitySchemes".
func updateComponentRef(ref *references.Reference, mappings map[string]string, componentType string) error {
	if ref == nil || len(mappings) == 0 {
		return nil
	}

	refStr := string(*ref)
	prefix := "#/components/" + componentType + "/"
	if !strings.HasPrefix(refStr, prefix) {
		return nil
	}

	// Extract component name from the reference
	componentName := strings.TrimPrefix(refStr, prefix)

	// Handle nested references (e.g., #/components/parameters/Foo/schema)
	if idx := strings.Index(componentName, "/"); idx >= 0 {
		baseName := componentName[:idx]
		suffix := componentName[idx:]
		if newName, exists := mappings[baseName]; exists {
			*ref = references.Reference(prefix + newName + suffix)
		}
	} else {
		if newName, exists := mappings[componentName]; exists {
			*ref = references.Reference(prefix + newName)
		}
	}

	return nil
}

// updateSecurityRequirements updates security requirement keys throughout the document
// based on the security scheme name mappings. It replaces SecurityRequirement objects
// rather than modifying them in place, because the underlying core model used for
// marshaling is not updated by high-level map operations.
// Covers document-level security, paths, and webhooks.
func updateSecurityRequirements(doc *openapi.OpenAPI, mappings map[string]string) {
	if doc == nil || len(mappings) == 0 {
		return
	}

	// Update document-level security
	doc.Security = remapSecurityArray(doc.Security, mappings)

	// Update operation-level security in paths
	if doc.Paths != nil {
		for _, pathItem := range doc.Paths.All() {
			remapPathItemOperationsSecurity(pathItem, mappings)
		}
	}

	// Update operation-level security in webhooks
	if doc.Webhooks != nil {
		for _, pathItem := range doc.Webhooks.All() {
			remapPathItemOperationsSecurity(pathItem, mappings)
		}
	}
}

// remapPathItemOperationsSecurity remaps security requirement keys for all operations
// in a single path item.
func remapPathItemOperationsSecurity(pathItem *openapi.ReferencedPathItem, mappings map[string]string) {
	if pathItem == nil || pathItem.Object == nil {
		return
	}
	for _, op := range pathItem.Object.All() {
		if op != nil && op.Security != nil {
			op.Security = remapSecurityArray(op.Security, mappings)
		}
	}
}

// remapSecurityArray creates a new security requirements array with remapped scheme names.
func remapSecurityArray(security []*openapi.SecurityRequirement, mappings map[string]string) []*openapi.SecurityRequirement {
	if len(security) == 0 || len(mappings) == 0 {
		return security
	}

	result := make([]*openapi.SecurityRequirement, len(security))
	for i, sr := range security {
		if sr == nil {
			result[i] = nil
			continue
		}

		var elems []*sequencedmap.Element[string, []string]
		for key, value := range sr.All() {
			newKey := key
			if mapped, ok := mappings[key]; ok {
				newKey = mapped
			}
			elems = append(elems, sequencedmap.NewElem(newKey, value))
		}
		result[i] = openapi.NewSecurityRequirement(elems...)
	}
	return result
}

// updateSchemaReferencesInDocument updates all $ref values pointing to schemas
// based on the provided mappings of originalName -> newName.
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
