package merge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	utils2 "github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/speakeasy-api/openapi/extensions"
	"github.com/speakeasy-api/openapi/jsonschema/oas3"
	"github.com/speakeasy-api/openapi/marshaller"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/overlay"
	"github.com/speakeasy-api/openapi/sequencedmap"
	"github.com/speakeasy-api/openapi/yml"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
)

// MergeOpenAPIDocuments merges multiple OpenAPI documents into a single document.
// This is the legacy function that maintains backward compatibility.
func MergeOpenAPIDocuments(ctx context.Context, inFiles []string, outFile string) error {
	inputs := make([]MergeInput, len(inFiles))
	for i, inFile := range inFiles {
		inputs[i] = MergeInput{Path: inFile}
	}

	return MergeOpenAPIDocumentsWithNamespaces(ctx, inputs, outFile, MergeOptions{
		YAMLOutput: utils2.HasYAMLExt(outFile),
	})
}

// MergeOpenAPIDocumentsWithNamespaces merges multiple OpenAPI documents with optional namespace support.
// When namespaces are provided, schema names are prefixed and x-speakeasy extensions are added.
func MergeOpenAPIDocumentsWithNamespaces(ctx context.Context, inputs []MergeInput, outFile string, opts MergeOptions) error {
	// Validate namespace consistency
	if err := validateNamespaces(inputs); err != nil {
		return err
	}

	inSchemas := make([][]byte, len(inputs))
	namespaces := make([]string, len(inputs))

	for i, input := range inputs {
		data, err := os.ReadFile(input.Path)
		if err != nil {
			return err
		}

		inSchemas[i] = data
		namespaces[i] = input.Namespace
	}

	mergedSchema, err := merge(ctx, inSchemas, namespaces, opts.YAMLOutput)
	if mergedSchema == nil {
		return err
	} else if err != nil {
		log.From(ctx).Warnf("Got warning(s) when merging:  %s\n\n", err.Error())
	}

	if err := os.WriteFile(outFile, mergedSchema, 0o644); err != nil {
		return err
	}

	return nil
}

func merge(ctx context.Context, inSchemas [][]byte, namespaces []string, yamlOut bool) ([]byte, error) {
	// Validate namespace consistency
	if err := validateNamespaceSlice(namespaces, len(inSchemas)); err != nil {
		return nil, err
	}

	var mergedDoc *openapi.OpenAPI
	var warnings []error
	state := newMergeState()

	// Track pre-namespace security for the merged doc so we can compare
	// originals (namespace prefixing makes equivalent schemes look different).
	var mergedOrigSec originalSecurityInfo

	for i, schema := range inSchemas {
		doc, err := loadOpenAPIDocument(ctx, schema)
		if err != nil {
			return nil, err
		}

		// Save the original (pre-namespace) security for later comparison.
		// This captures both the security requirements and the scheme definitions
		// they reference, so we can detect cases where two specs use the same
		// scheme name but with different definitions.
		origSec := captureOriginalSecurity(doc)

		// Apply namespace if provided
		namespace := ""
		if i < len(namespaces) {
			namespace = namespaces[i]
		}

		if namespace != "" {
			// Apply namespace prefixes to all component types
			schemaMappings := applyNamespaceToSchemas(doc, namespace)
			paramMappings := applyNamespaceToParameters(doc, namespace)
			responseMappings := applyNamespaceToResponses(doc, namespace)
			requestBodyMappings := applyNamespaceToRequestBodies(doc, namespace)
			headerMappings := applyNamespaceToHeaders(doc, namespace)
			secSchemeMappings := applyNamespaceToSecuritySchemes(doc, namespace)

			// Update all references (schemas + components) in a single document walk
			if err := updateAllReferencesInDocument(ctx, doc, schemaMappings, namespaceMappings{
				Parameters:      paramMappings,
				Responses:       responseMappings,
				RequestBodies:   requestBodyMappings,
				Headers:         headerMappings,
				SecuritySchemes: secSchemeMappings,
			}); err != nil {
				return nil, fmt.Errorf("failed to update references for namespace %s: %w", namespace, err)
			}

			// Update security requirement keys to match renamed security schemes
			updateSecurityRequirements(doc, secSchemeMappings)
		}

		if mergedDoc == nil {
			mergedDoc = doc
			mergedOrigSec = origSec
			initMergeState(state, doc, namespace)
			continue
		}

		var errs []error
		mergedDoc, mergedOrigSec, errs = mergeDocumentsWithState(state, mergedDoc, doc, mergedOrigSec, origSec, namespace, i+1)
		warnings = append(warnings, errs...)
	}

	if mergedDoc == nil {
		return nil, errors.New("no documents to merge")
	}

	// Post-merge: deduplicate operationIds
	deduplicateOperationIds(state, mergedDoc)

	// Post-merge: collapse namespaced components that are equivalent
	// (ignoring description/summary differences)
	deduplicateEquivalentComponents(mergedDoc)

	// Post-merge: normalize operation-level tag references to match
	// the chosen document-level tag names (case-insensitive)
	normalizeOperationTags(mergedDoc)

	buf := bytes.NewBuffer(nil)
	var err error

	// Set output format on the document's core config
	if core := mergedDoc.GetCore(); core != nil {
		config := core.Config
		if config == nil {
			config = yml.GetDefaultConfig()
		}
		if yamlOut {
			config.OutputFormat = yml.OutputFormatYAML
		} else {
			config.OutputFormat = yml.OutputFormatJSON
		}
		core.SetConfig(config)
	}

	err = openapi.Marshal(ctx, mergedDoc, buf)
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		return buf.Bytes(), multierror.Append(nil, warnings...)
	}

	return buf.Bytes(), nil
}

// loadOpenAPIDocument loads an OpenAPI document using the speakeasy-api/openapi parser.
func loadOpenAPIDocument(ctx context.Context, data []byte) (*openapi.OpenAPI, error) {
	doc, validationErrs, err := openapi.Unmarshal(ctx, bytes.NewReader(data), openapi.WithSkipValidation())
	if err != nil {
		return nil, err
	}

	// Log validation errors but don't fail - let the merge proceed
	for _, validationErr := range validationErrs {
		log.From(ctx).Warn(fmt.Sprintf("validation warning: %s", validationErr.Error()))
	}

	// Check if it's OpenAPI 3.x
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil, errors.New("only OpenAPI 3.x is supported")
	}

	return doc, nil
}

// MergeDocuments merges two OpenAPI documents into one.
// This is the public backward-compatible API. For namespace-aware merging,
// the internal mergeDocumentsWithState is used instead.
func MergeDocuments(mergedDoc, doc *openapi.OpenAPI) (*openapi.OpenAPI, []error) {
	state := newMergeState()
	initMergeState(state, mergedDoc, "")
	merged, _, errs := mergeDocumentsWithState(state, mergedDoc, doc, captureOriginalSecurity(mergedDoc), captureOriginalSecurity(doc), "", 2)
	deduplicateOperationIds(state, merged)
	normalizeOperationTags(merged)
	return merged, errs
}

// mergeDocumentsWithState merges two OpenAPI documents with namespace-aware
// tag deduplication, path/method conflict disambiguation, and operationId tracking.
// mergedOrigSec and docOrigSec are the pre-namespace security info, used to detect
// whether the two documents truly have different effective security.
func mergeDocumentsWithState(state *mergeState, mergedDoc, doc *openapi.OpenAPI, mergedOrigSec, docOrigSec originalSecurityInfo, docNamespace string, docCounter int) (*openapi.OpenAPI, originalSecurityInfo, []error) {
	mergedVersion, _ := version.NewSemver(mergedDoc.OpenAPI)
	docVersion, _ := version.NewSemver(doc.OpenAPI)
	errs := make([]error, 0)

	if mergedVersion == nil || docVersion != nil && docVersion.GreaterThan(mergedVersion) {
		mergedDoc.OpenAPI = doc.OpenAPI
	}

	// Merge Info - last wins for most fields, but append description and summary
	mergedDoc.Info = mergeInfo(mergedDoc.Info, doc.Info)

	// Merge Extensions
	if doc.Extensions != nil {
		var extErrors []error
		mergedDoc.Extensions, extErrors = mergeExtensions(mergedDoc.Extensions, doc.Extensions)
		for _, err := range extErrors {
			errs = append(errs, fmt.Errorf("%w in global extension", err))
		}
	}

	// Merge Servers
	mergedServers, opServers := mergeServers(mergedDoc.Servers, doc.Servers, true)
	if len(opServers) > 0 {
		setOperationServers(mergedDoc, mergedDoc.Servers)
		setOperationServers(doc, opServers)
		mergedDoc.Servers = nil
	} else {
		mergedDoc.Servers = mergedServers
	}

	// Merge Security - inline to operations when global security differs between documents
	// (analogous to how servers are handled when they conflict).
	// We compare the pre-namespace originals because namespace prefixing makes
	// equivalent schemes look different (svcA_bearer vs svcB_bearer).
	mergedDoc.Security, doc.Security, mergedOrigSec = mergeSecurity(mergedDoc, doc, mergedOrigSec, docOrigSec)

	// Merge Tags (case-insensitive with content-aware disambiguation)
	tagResult := mergeTagsWithState(state, mergedDoc, doc, docNamespace, docCounter)
	// Update operation-level tag references in each doc using its own rename map
	// (case-insensitive matching, per-document maps avoid ambiguity)
	updateOperationTagRefs(mergedDoc, tagResult.existingRenames)
	updateOperationTagRefs(doc, tagResult.incomingRenames)

	// Merge Paths (with method-level conflict detection and fragment disambiguation)
	pathErrs := mergePathsWithState(state, mergedDoc, doc, docNamespace, docCounter)
	errs = append(errs, pathErrs...)

	// Merge Components
	if doc.Components != nil {
		if mergedDoc.Components == nil {
			mergedDoc.Components = doc.Components
		} else {
			var componentErrs []error
			mergedDoc.Components, componentErrs = mergeComponents(mergedDoc.Components, doc.Components)
			errs = append(errs, componentErrs...)
		}
	}

	// Merge Webhooks
	if doc.Webhooks != nil {
		if mergedDoc.Webhooks == nil {
			mergedDoc.Webhooks = doc.Webhooks
		} else {
			for path, webhook := range doc.Webhooks.All() {
				if _, ok := mergedDoc.Webhooks.Get(path); !ok {
					mergedDoc.Webhooks.Set(path, webhook)
				} else {
					mergedWebhook, _ := mergedDoc.Webhooks.Get(path)
					pi, pathItemErrs := mergeReferencedPathItems(mergedWebhook, webhook)
					mergedDoc.Webhooks.Set(path, pi)
					errs = append(errs, pathItemErrs...)
				}
			}
		}
	}

	// Merge ExternalDocs
	if doc.ExternalDocs != nil {
		mergedDoc.ExternalDocs = doc.ExternalDocs
	}

	return mergedDoc, mergedOrigSec, errs
}

// mergeInfo merges two Info objects. Most fields use last-wins semantics,
// but Description and Summary are appended (with a newline separator) so that
// content from all merged documents is preserved.
func mergeInfo(merged, incoming openapi.Info) openapi.Info {
	existingDesc := derefStr(merged.Description)
	existingSummary := derefStr(merged.Summary)

	// Take the incoming info (last wins for most fields)
	result := incoming

	// Append descriptions across documents
	result.Description = appendStrPtrs(existingDesc, derefStr(incoming.Description))

	// Append summaries across documents
	result.Summary = appendStrPtrs(existingSummary, derefStr(incoming.Summary))

	return result
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func appendStrPtrs(a, b string) *string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	switch {
	case a != "" && b != "" && a != b:
		combined := a + "\n" + b
		return &combined
	case a != "":
		return &a
	case b != "":
		return &b
	default:
		return nil
	}
}

func mergeReferencedPathItems(mergedPathItem, pathItem *openapi.ReferencedPathItem) (*openapi.ReferencedPathItem, []error) {
	if pathItem == nil {
		return mergedPathItem, nil
	}
	if mergedPathItem == nil {
		return pathItem, nil
	}

	// If either is a reference, prefer the new one
	if pathItem.Object == nil {
		return pathItem, nil
	}
	if mergedPathItem.Object == nil {
		mergedPathItem.Object = pathItem.Object
		return mergedPathItem, nil
	}

	// Both have objects, merge them
	merged, errs := mergePathItemObjects(mergedPathItem.Object, pathItem.Object)
	mergedPathItem.Object = merged
	return mergedPathItem, errs
}

func mergePathItemObjects(mergedPathItem, pathItem *openapi.PathItem) (*openapi.PathItem, []error) {
	var errs []error

	// Merge operations
	for method, op := range pathItem.All() {
		mergedPathItem.Set(method, op)
	}

	// Merge Summary
	if pathItem.Summary != nil && *pathItem.Summary != "" {
		mergedPathItem.Summary = pathItem.Summary
	}

	// Merge Description
	if pathItem.Description != nil && *pathItem.Description != "" {
		mergedPathItem.Description = pathItem.Description
	}

	// Merge Parameters
	mergedPathItem.Parameters = mergeParameters(mergedPathItem.Parameters, pathItem.Parameters)

	// Merge Servers
	mergedPathItem.Servers, _ = mergeServers(mergedPathItem.Servers, pathItem.Servers, false)

	// Merge Extensions
	if pathItem.Extensions != nil {
		var extErrs []error
		mergedPathItem.Extensions, extErrs = mergeExtensions(mergedPathItem.Extensions, pathItem.Extensions)
		errs = append(errs, extErrs...)
	}

	return mergedPathItem, errs
}

func mergeServers(mergedServers, servers []*openapi.Server, global bool) ([]*openapi.Server, []*openapi.Server) {
	if len(mergedServers) == 0 {
		return servers, nil
	}

	if len(servers) > 0 {
		mergeable := !global

		if len(mergedServers) > 0 {
			for _, server := range servers {
				for _, mergedServer := range mergedServers {
					if mergedServer.URL == server.URL {
						mergeable = true
					}
				}
			}
		} else {
			mergeable = true
		}

		if !mergeable {
			return nil, servers
		}

		for _, server := range servers {
			replaced := false
			for i, mergedServer := range mergedServers {
				if mergedServer.URL == server.URL {
					mergedServers[i] = server
					replaced = true
					break
				}
			}
			if !replaced {
				mergedServers = append(mergedServers, server)
			}
		}
	}

	return mergedServers, nil
}

func mergeParameters(mergedParameters, parameters []*openapi.ReferencedParameter) []*openapi.ReferencedParameter {
	if len(mergedParameters) == 0 {
		return parameters
	}

	if len(parameters) > 0 {
		for _, parameter := range parameters {
			replaced := false
			paramName := getParameterName(parameter)

			for i, mergedParameter := range mergedParameters {
				if getParameterName(mergedParameter) == paramName {
					mergedParameters[i] = parameter
					replaced = true
					break
				}
			}

			if !replaced {
				mergedParameters = append(mergedParameters, parameter)
			}
		}
	}

	return mergedParameters
}

func getParameterName(param *openapi.ReferencedParameter) string {
	if param == nil {
		return ""
	}
	if param.Object != nil {
		return param.Object.Name
	}
	if param.Reference != nil {
		return string(*param.Reference)
	}
	return ""
}

func mergeComponents(mergedComponents, components *openapi.Components) (*openapi.Components, []error) {
	errs := make([]error, 0)

	// Merge Schemas
	if components.Schemas != nil {
		if mergedComponents.Schemas == nil {
			mergedComponents.Schemas = components.Schemas
		} else {
			for name, schema := range components.Schemas.All() {
				existing, exists := mergedComponents.Schemas.Get(name)
				if exists {
					if err := isSchemaEquivalent(existing, schema); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Schemas.Set(name, schema)
			}
		}
	}

	// Merge Responses
	if components.Responses != nil {
		if mergedComponents.Responses == nil {
			mergedComponents.Responses = components.Responses
		} else {
			for name, response := range components.Responses.All() {
				existing, exists := mergedComponents.Responses.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, response); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Responses.Set(name, response)
			}
		}
	}

	// Merge Parameters
	if components.Parameters != nil {
		if mergedComponents.Parameters == nil {
			mergedComponents.Parameters = components.Parameters
		} else {
			for name, parameter := range components.Parameters.All() {
				existing, exists := mergedComponents.Parameters.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, parameter); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Parameters.Set(name, parameter)
			}
		}
	}

	// Merge Examples
	if components.Examples != nil {
		if mergedComponents.Examples == nil {
			mergedComponents.Examples = components.Examples
		} else {
			for name, example := range components.Examples.All() {
				existing, exists := mergedComponents.Examples.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, example); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Examples.Set(name, example)
			}
		}
	}

	// Merge RequestBodies
	if components.RequestBodies != nil {
		if mergedComponents.RequestBodies == nil {
			mergedComponents.RequestBodies = components.RequestBodies
		} else {
			for name, requestBody := range components.RequestBodies.All() {
				existing, exists := mergedComponents.RequestBodies.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, requestBody); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.RequestBodies.Set(name, requestBody)
			}
		}
	}

	// Merge Headers
	if components.Headers != nil {
		if mergedComponents.Headers == nil {
			mergedComponents.Headers = components.Headers
		} else {
			for name, header := range components.Headers.All() {
				existing, exists := mergedComponents.Headers.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, header); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Headers.Set(name, header)
			}
		}
	}

	// Merge SecuritySchemes
	if components.SecuritySchemes != nil {
		if mergedComponents.SecuritySchemes == nil {
			mergedComponents.SecuritySchemes = components.SecuritySchemes
		} else {
			for name, securityScheme := range components.SecuritySchemes.All() {
				existing, exists := mergedComponents.SecuritySchemes.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, securityScheme); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.SecuritySchemes.Set(name, securityScheme)
			}
		}
	}

	// Merge Links
	if components.Links != nil {
		if mergedComponents.Links == nil {
			mergedComponents.Links = components.Links
		} else {
			for name, link := range components.Links.All() {
				existing, exists := mergedComponents.Links.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, link); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Links.Set(name, link)
			}
		}
	}

	// Merge Callbacks
	if components.Callbacks != nil {
		if mergedComponents.Callbacks == nil {
			mergedComponents.Callbacks = components.Callbacks
		} else {
			for name, callback := range components.Callbacks.All() {
				existing, exists := mergedComponents.Callbacks.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, callback); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.Callbacks.Set(name, callback)
			}
		}
	}

	// Merge PathItems
	if components.PathItems != nil {
		if mergedComponents.PathItems == nil {
			mergedComponents.PathItems = components.PathItems
		} else {
			for name, pathItem := range components.PathItems.All() {
				existing, exists := mergedComponents.PathItems.Get(name)
				if exists {
					if err := isReferencedEquivalent(existing, pathItem); err != nil {
						errs = append(errs, err)
					}
				}
				mergedComponents.PathItems.Set(name, pathItem)
			}
		}
	}

	// Merge Extensions
	if components.Extensions != nil {
		var extensionErrs []error
		mergedComponents.Extensions, extensionErrs = mergeExtensions(mergedComponents.Extensions, components.Extensions)
		errs = append(errs, extensionErrs...)
	}

	return mergedComponents, errs
}

// descriptiveFields are fields that should be ignored when comparing components
// for equivalence during merging. Two components that differ only in these fields
// are considered equivalent (e.g. security schemes with different descriptions
// but the same type/scheme/bearerFormat).
var descriptiveFields = map[string]bool{
	"description": true,
	"summary":     true,
}

// stripDescriptiveFields removes description and summary keys from a yaml.Node
// tree so that they don't cause false conflicts during merge comparison.
func stripDescriptiveFields(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			stripDescriptiveFields(child)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
		filtered := make([]*yaml.Node, 0, len(node.Content))
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind == yaml.ScalarNode && descriptiveFields[key.Value] {
				continue
			}
			stripDescriptiveFields(value)
			filtered = append(filtered, key, value)
		}
		node.Content = filtered
		return
	}

	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			stripDescriptiveFields(child)
		}
	}
}

// isSchemaEquivalent checks if two schemas are equivalent
func isSchemaEquivalent(a, b *oas3.JSONSchema[oas3.Referenceable]) error {
	if a == nil || b == nil {
		return nil
	}

	// Marshal both to YAML and compare
	ctx := context.Background()
	bufA := bytes.NewBuffer(nil)
	bufB := bytes.NewBuffer(nil)

	if err := marshaller.Marshal(ctx, a, bufA); err != nil {
		return fmt.Errorf("error marshalling schema a: %w", err)
	}
	if err := marshaller.Marshal(ctx, b, bufB); err != nil {
		return fmt.Errorf("error marshalling schema b: %w", err)
	}

	var nodeA, nodeB yaml.Node
	if err := yaml.Unmarshal(bufA.Bytes(), &nodeA); err != nil {
		return fmt.Errorf("error unmarshalling schema a: %w", err)
	}
	if err := yaml.Unmarshal(bufB.Bytes(), &nodeB); err != nil {
		return fmt.Errorf("error unmarshalling schema b: %w", err)
	}

	// Strip description/summary so they don't cause false conflicts
	stripDescriptiveFields(&nodeA)
	stripDescriptiveFields(&nodeB)

	nodeOverlay, err := overlay.Compare("comparison between schemas", &nodeA, nodeB)
	if err != nil {
		return fmt.Errorf("error comparing schemas: %w", err)
	}

	if len(nodeOverlay.Actions) > 0 {
		return fmt.Errorf("schemas are not equivalent: \nSchema 1 = %s\n\n Schema 2 = %s", bufA.String(), bufB.String())
	}

	return nil
}

// isReferencedEquivalent checks if two referenced objects are equivalent using YAML comparison
func isReferencedEquivalent[T any](a, b *T) error {
	if a == nil || b == nil {
		return nil
	}

	// Use reflect.DeepEqual for simple comparison
	if reflect.DeepEqual(a, b) {
		return nil
	}

	// Marshal both to YAML and compare using yaml.Marshal directly
	// (marshaller.Marshal requires a specific interface that generics don't satisfy)
	bytesA, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("error marshalling a: %w", err)
	}
	bytesB, err := yaml.Marshal(b)
	if err != nil {
		return fmt.Errorf("error marshalling b: %w", err)
	}

	var nodeA, nodeB yaml.Node
	if err := yaml.Unmarshal(bytesA, &nodeA); err != nil {
		return fmt.Errorf("error unmarshalling a: %w", err)
	}
	if err := yaml.Unmarshal(bytesB, &nodeB); err != nil {
		return fmt.Errorf("error unmarshalling b: %w", err)
	}

	// Strip description/summary so they don't cause false conflicts
	stripDescriptiveFields(&nodeA)
	stripDescriptiveFields(&nodeB)

	nodeOverlay, err := overlay.Compare("comparison between objects", &nodeA, nodeB)
	if err != nil {
		return fmt.Errorf("error comparing objects: %w", err)
	}

	if len(nodeOverlay.Actions) > 0 {
		return fmt.Errorf("objects are not equivalent: \nObject 1 = %s\n\n Object 2 = %s", string(bytesA), string(bytesB))
	}

	return nil
}

func mergeExtensions(mergedExtensions, exts *extensions.Extensions) (*extensions.Extensions, []error) {
	if mergedExtensions == nil {
		return exts, nil
	}
	if exts == nil {
		return mergedExtensions, nil
	}

	errs := make([]error, 0)

	for name, extYamlNode := range exts.All() {
		var ext any
		if extYamlNode != nil {
			_ = extYamlNode.Decode(&ext)
		}

		if ext2YamlNode, ok := mergedExtensions.Get(name); ok {
			var ext2 any
			if ext2YamlNode != nil {
				_ = ext2YamlNode.Decode(&ext2)
			}

			if !reflect.DeepEqual(ext, ext2) {
				errs = append(errs, fmt.Errorf("conflicting extension %#v %#v", ext, ext2))
			}
		}

		mergedExtensions.Set(name, extYamlNode)
	}

	return mergedExtensions, errs
}

// deduplicateEquivalentComponents collapses namespaced components that are
// equivalent (ignoring description/summary). For example, if svcA_bearerAuth
// and svcB_bearerAuth are identical security schemes except for description,
// they are collapsed into a single bearerAuth entry.
func deduplicateEquivalentComponents(doc *openapi.OpenAPI) {
	if doc == nil || doc.Components == nil {
		return
	}

	deduplicateSecuritySchemes(doc)
}

// getNameOverride reads the x-speakeasy-name-override extension value from an extensions map.
func getNameOverride(exts *extensions.Extensions) string {
	if exts == nil {
		return ""
	}
	node, ok := exts.Get("x-speakeasy-name-override")
	if !ok || node == nil {
		return ""
	}
	return node.Value
}

// isEquivalentIgnoringDescriptiveAndNamespaceFields checks if two objects are
// equivalent after stripping description, summary, and x-speakeasy-* extension fields.
func isEquivalentIgnoringDescriptiveAndNamespaceFields[T any](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	bytesA, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bytesB, err := yaml.Marshal(b)
	if err != nil {
		return false
	}

	var nodeA, nodeB yaml.Node
	if err := yaml.Unmarshal(bytesA, &nodeA); err != nil {
		return false
	}
	if err := yaml.Unmarshal(bytesB, &nodeB); err != nil {
		return false
	}

	stripDescriptiveFields(&nodeA)
	stripDescriptiveFields(&nodeB)
	stripSpeakeasyExtensions(&nodeA)
	stripSpeakeasyExtensions(&nodeB)

	nodeOverlay, err := overlay.Compare("equivalence check", &nodeA, nodeB)
	if err != nil {
		return false
	}
	return len(nodeOverlay.Actions) == 0
}

// stripSpeakeasyExtensions removes x-speakeasy-name-override and
// x-speakeasy-model-namespace from yaml mapping nodes.
func stripSpeakeasyExtensions(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			stripSpeakeasyExtensions(child)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
		filtered := make([]*yaml.Node, 0, len(node.Content))
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind == yaml.ScalarNode &&
				(key.Value == "x-speakeasy-name-override" || key.Value == "x-speakeasy-model-namespace") {
				continue
			}
			stripSpeakeasyExtensions(value)
			filtered = append(filtered, key, value)
		}
		node.Content = filtered
		return
	}

	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			stripSpeakeasyExtensions(child)
		}
	}
}

// schemeEntry pairs a namespaced component name with its security scheme.
// Used during deduplication to track grouped entries.
type schemeEntry struct {
	namespacedName string
	scheme         *openapi.ReferencedSecurityScheme
}

// areMergeableSecuritySchemes checks whether two security schemes can be
// collapsed into one during deduplication. The check is type-aware:
//   - oauth2: mergeable if same flow types present with matching URLs (scopes may differ)
//   - http: mergeable if same scheme and bearerFormat
//   - apiKey: mergeable if same name and in
//   - openIdConnect: mergeable if same openIdConnectUrl
//   - mutualTLS: always mergeable
func areMergeableSecuritySchemes(a, b *openapi.SecurityScheme) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Type != b.Type {
		return false
	}

	switch a.Type {
	case openapi.SecuritySchemeTypeOAuth2:
		return areMergeableOAuth2Schemes(a, b)
	case openapi.SecuritySchemeTypeHTTP:
		return a.GetScheme() == b.GetScheme() && a.GetBearerFormat() == b.GetBearerFormat()
	case openapi.SecuritySchemeTypeAPIKey:
		return a.GetName() == b.GetName() && a.GetIn() == b.GetIn()
	case openapi.SecuritySchemeTypeOpenIDConnect:
		return a.GetOpenIdConnectUrl() == b.GetOpenIdConnectUrl()
	case openapi.SecuritySchemeTypeMutualTLS:
		return true
	default:
		return isEquivalentIgnoringDescriptiveAndNamespaceFields(a, b)
	}
}

// areMergeableOAuth2Schemes checks whether two oauth2 security schemes have
// the same flow types present with matching URLs per flow. Scopes and
// descriptions are allowed to differ.
func areMergeableOAuth2Schemes(a, b *openapi.SecurityScheme) bool {
	af, bf := a.Flows, b.Flows
	if (af == nil) != (bf == nil) {
		return false
	}
	if af == nil {
		return true
	}

	// Check that both schemes have the same set of flows present
	if (af.Implicit == nil) != (bf.Implicit == nil) ||
		(af.Password == nil) != (bf.Password == nil) ||
		(af.ClientCredentials == nil) != (bf.ClientCredentials == nil) ||
		(af.AuthorizationCode == nil) != (bf.AuthorizationCode == nil) ||
		(af.DeviceAuthorization == nil) != (bf.DeviceAuthorization == nil) {
		return false
	}

	// For each present flow, check that URLs match
	if af.Implicit != nil && !oauthFlowURLsMatch(af.Implicit, bf.Implicit) {
		return false
	}
	if af.Password != nil && !oauthFlowURLsMatch(af.Password, bf.Password) {
		return false
	}
	if af.ClientCredentials != nil && !oauthFlowURLsMatch(af.ClientCredentials, bf.ClientCredentials) {
		return false
	}
	if af.AuthorizationCode != nil && !oauthFlowURLsMatch(af.AuthorizationCode, bf.AuthorizationCode) {
		return false
	}
	if af.DeviceAuthorization != nil && !oauthFlowURLsMatch(af.DeviceAuthorization, bf.DeviceAuthorization) {
		return false
	}

	return true
}

// oauthFlowURLsMatch checks whether two OAuth flows have identical URLs.
func oauthFlowURLsMatch(a, b *openapi.OAuthFlow) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.GetAuthorizationURL() == b.GetAuthorizationURL() &&
		a.GetTokenURL() == b.GetTokenURL() &&
		a.GetRefreshURL() == b.GetRefreshURL() &&
		a.GetDeviceAuthorizationURL() == b.GetDeviceAuthorizationURL()
}

// mergeSecuritySchemeDescriptions appends descriptions from all entries using
// the same deduplicating newline-separated pattern used for info descriptions.
func mergeSecuritySchemeDescriptions(winner *openapi.SecurityScheme, entries []schemeEntry) {
	combined := ""
	for _, entry := range entries {
		desc := entry.scheme.Object.GetDescription()
		combined = derefStr(appendStrPtrs(combined, desc))
	}
	if combined != "" {
		winner.Description = &combined
	}
}

// mergeOAuth2Scopes unions the scopes from all entries into the winner's flows.
// Only operates on oauth2 security schemes. A fresh scopes map is built from
// all entries in order so that the merged result is deterministic. For
// duplicate scope keys the last description wins.
func mergeOAuth2Scopes(winner *openapi.SecurityScheme, entries []schemeEntry) {
	if winner.Type != openapi.SecuritySchemeTypeOAuth2 || winner.Flows == nil {
		return
	}

	type flowPair struct {
		winnerFlow *openapi.OAuthFlow
		getFlow    func(*openapi.OAuthFlows) *openapi.OAuthFlow
	}

	pairs := []flowPair{
		{winner.Flows.Implicit, (*openapi.OAuthFlows).GetImplicit},
		{winner.Flows.Password, (*openapi.OAuthFlows).GetPassword},
		{winner.Flows.ClientCredentials, (*openapi.OAuthFlows).GetClientCredentials},
		{winner.Flows.AuthorizationCode, (*openapi.OAuthFlows).GetAuthorizationCode},
		{winner.Flows.DeviceAuthorization, (*openapi.OAuthFlows).GetDeviceAuthorization},
	}

	for _, pair := range pairs {
		wf := pair.winnerFlow
		if wf == nil {
			continue
		}

		// Build a fresh scopes map from all entries in order.
		// For duplicate keys Set updates in-place (last wins for description,
		// position preserved from first occurrence).
		merged := sequencedmap.New[string, string]()
		for _, entry := range entries {
			obj := entry.scheme.Object
			if obj.Type != openapi.SecuritySchemeTypeOAuth2 || obj.Flows == nil {
				continue
			}
			ef := pair.getFlow(obj.Flows)
			if ef == nil || ef.Scopes == nil {
				continue
			}
			for scopeName, scopeDesc := range ef.Scopes.All() {
				merged.Set(scopeName, scopeDesc)
			}
		}
		wf.Scopes = merged
	}
}

// deduplicateSecuritySchemes collapses namespaced security schemes that are
// mergeable into a single entry. For oauth2 schemes this includes unioning
// scopes; for all types descriptions are appended. Updates security
// requirements throughout the document to reference the collapsed name.
//
// Groups are formed using case-insensitive matching on the original name
// (x-speakeasy-name-override), so "BearerAuth" and "bearerAuth" are treated
// as the same group. Within a group, schemes are partitioned into subgroups
// of mutually-mergeable entries (by type-aware checks), so a mixed group
// containing both http and oauth2 schemes will merge each type independently
// rather than blocking all deduplication.
func deduplicateSecuritySchemes(doc *openapi.OpenAPI) {
	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		return
	}

	// Group namespaced schemes by their case-insensitive original name
	groups := sequencedmap.New[string, []schemeEntry]()

	for name, scheme := range doc.Components.SecuritySchemes.All() {
		if scheme == nil || scheme.IsReference() || scheme.Object == nil {
			continue
		}
		override := getNameOverride(scheme.Object.Extensions)
		if override == "" {
			continue // not a namespaced component
		}
		key := strings.ToLower(override)
		existing, _ := groups.Get(key)
		groups.Set(key, append(existing, schemeEntry{namespacedName: name, scheme: scheme}))
	}

	// For each group, partition into mergeable subgroups and process
	renameMappings := make(map[string]string) // namespacedName -> canonicalName
	removals := make(map[string]bool)

	for _, entries := range groups.All() {

		if len(entries) == 1 {
			// Unique scheme (only in one document): strip namespace, rename back to original
			entry := entries[0]
			originalName := getNameOverride(entry.scheme.Object.Extensions)
			renameMappings[entry.namespacedName] = originalName
			if entry.scheme.Object.Extensions != nil {
				entry.scheme.Object.Extensions.Delete("x-speakeasy-name-override")
				entry.scheme.Object.Extensions.Delete("x-speakeasy-model-namespace")
			}
			continue
		}

		// Partition into subgroups of mutually-mergeable schemes
		subgroups := partitionMergeableSchemes(entries)

		// Determine canonical names for each subgroup and resolve conflicts,
		// then merge each subgroup that claims a name.
		processSchemeSubgroups(subgroups, renameMappings, removals)
	}

	if len(renameMappings) == 0 {
		return
	}

	// Rebuild the security schemes map: remove duplicates, rename winner
	newSchemes := sequencedmap.New[string, *openapi.ReferencedSecurityScheme]()
	for name, scheme := range doc.Components.SecuritySchemes.All() {
		if removals[name] {
			continue
		}
		if newName, ok := renameMappings[name]; ok {
			newSchemes.Set(newName, scheme)
		} else {
			newSchemes.Set(name, scheme)
		}
	}
	doc.Components.SecuritySchemes = newSchemes

	// Update security requirements throughout the document
	updateSecurityRequirements(doc, renameMappings)
}

// partitionMergeableSchemes groups entries into subgroups where all members
// within a subgroup are mutually mergeable. Uses greedy placement: each entry
// is added to the first compatible subgroup.
func partitionMergeableSchemes(entries []schemeEntry) [][]schemeEntry {
	var subgroups [][]schemeEntry
	for _, entry := range entries {
		placed := false
		for i, sg := range subgroups {
			if areMergeableSecuritySchemes(sg[0].scheme.Object, entry.scheme.Object) {
				subgroups[i] = append(subgroups[i], entry)
				placed = true
				break
			}
		}
		if !placed {
			subgroups = append(subgroups, []schemeEntry{entry})
		}
	}
	return subgroups
}

// preferredOverrideName returns the most common x-speakeasy-name-override value
// among the entries. Ties are broken by first occurrence (document order).
func preferredOverrideName(entries []schemeEntry) string {
	counts := sequencedmap.New[string, int]()
	for _, entry := range entries {
		name := getNameOverride(entry.scheme.Object.Extensions)
		counts.Set(name, counts.GetOrZero(name)+1)
	}
	best := ""
	bestCount := 0
	for name, count := range counts.All() {
		if best == "" || count > bestCount {
			best = name
			bestCount = count
		}
	}
	return best
}

// processSchemeSubgroups handles naming and merging for partitioned subgroups.
// Each subgroup picks a canonical name (most common original override name).
// If multiple subgroups want the same case-sensitive name, the largest wins;
// if tied, all stay namespaced (conservative). Subgroups that claim a name are
// merged (descriptions appended, oauth2 scopes unioned).
func processSchemeSubgroups(subgroups [][]schemeEntry, renameMappings map[string]string, removals map[string]bool) {
	type subgroupInfo struct {
		entries []schemeEntry
		name    string // canonical name, or "" if staying namespaced
	}

	infos := make([]subgroupInfo, len(subgroups))
	for i, sg := range subgroups {
		infos[i] = subgroupInfo{entries: sg, name: preferredOverrideName(sg)}
	}

	// Resolve name conflicts: group subgroups by desired canonical name
	nameClaims := sequencedmap.New[string, []int]() // canonical name -> indices into infos
	for i, info := range infos {
		existing, _ := nameClaims.Get(info.name)
		nameClaims.Set(info.name, append(existing, i))
	}
	for _, indices := range nameClaims.All() {
		if len(indices) <= 1 {
			continue
		}
		// Multiple subgroups want the same name: largest wins, ties all stay namespaced
		maxSize := 0
		for _, idx := range indices {
			if len(infos[idx].entries) > maxSize {
				maxSize = len(infos[idx].entries)
			}
		}
		winnerCount := 0
		winnerIdx := -1
		for _, idx := range indices {
			if len(infos[idx].entries) == maxSize {
				winnerCount++
				winnerIdx = idx
			}
		}
		if winnerCount == 1 {
			// Clear winner: all others lose their name
			for _, idx := range indices {
				if idx != winnerIdx {
					infos[idx].name = ""
				}
			}
		} else {
			// Tie: all stay namespaced
			for _, idx := range indices {
				infos[idx].name = ""
			}
		}
	}

	// Process each subgroup
	for _, info := range infos {
		if info.name == "" {
			// Subgroup lost the canonical name conflict — still merge internally
			// under the last entry's namespaced name so duplicates are collapsed.
			if len(info.entries) > 1 {
				winner := info.entries[len(info.entries)-1]
				mergeSecuritySchemeDescriptions(winner.scheme.Object, info.entries)
				mergeOAuth2Scopes(winner.scheme.Object, info.entries)
				for _, entry := range info.entries {
					if entry.namespacedName != winner.namespacedName {
						renameMappings[entry.namespacedName] = winner.namespacedName
						removals[entry.namespacedName] = true
					}
				}
				if winner.scheme.Object.Extensions != nil {
					winner.scheme.Object.Extensions.Delete("x-speakeasy-name-override")
					winner.scheme.Object.Extensions.Delete("x-speakeasy-model-namespace")
				}
			}
			continue
		}

		if len(info.entries) == 1 {
			// Single entry: strip namespace, rename to canonical name
			entry := info.entries[0]
			renameMappings[entry.namespacedName] = info.name
			if entry.scheme.Object.Extensions != nil {
				entry.scheme.Object.Extensions.Delete("x-speakeasy-name-override")
				entry.scheme.Object.Extensions.Delete("x-speakeasy-model-namespace")
			}
			continue
		}

		// Multiple entries: merge into last entry (winner)
		winner := info.entries[len(info.entries)-1]

		mergeSecuritySchemeDescriptions(winner.scheme.Object, info.entries)
		mergeOAuth2Scopes(winner.scheme.Object, info.entries)

		for _, entry := range info.entries {
			renameMappings[entry.namespacedName] = info.name
			if entry.namespacedName != winner.namespacedName {
				removals[entry.namespacedName] = true
			}
		}

		// Clean up extensions from the winner
		if winner.scheme.Object.Extensions != nil {
			winner.scheme.Object.Extensions.Delete("x-speakeasy-name-override")
			winner.scheme.Object.Extensions.Delete("x-speakeasy-model-namespace")
		}
	}
}

func setOperationServers(doc *openapi.OpenAPI, opServers []*openapi.Server) {
	if doc.Paths == nil {
		return
	}

	for _, pathItem := range doc.Paths.All() {
		if pathItem.Object == nil {
			continue
		}

		for _, op := range pathItem.Object.All() {
			if op != nil && len(op.Servers) == 0 {
				op.Servers = append([]*openapi.Server{}, opServers...)
			}
		}
	}
}

// securityRequirementsEqual checks whether two global security requirement lists are
// semantically equivalent (same scheme names with same scopes, in any order).
func securityRequirementsEqual(a, b []*openapi.SecurityRequirement) bool {
	if len(a) != len(b) {
		return false
	}

	// Build a comparable representation for each side.
	type entry struct {
		name   string
		scopes string
	}
	toSet := func(reqs []*openapi.SecurityRequirement) map[entry]int {
		m := make(map[entry]int)
		for _, req := range reqs {
			if req == nil {
				continue
			}
			for k, v := range req.All() {
				scopes := make([]string, len(v))
				copy(scopes, v)
				// Sort for deterministic comparison.
				sort.Strings(scopes)
				m[entry{name: k, scopes: strings.Join(scopes, ",")}]++
			}
		}
		return m
	}

	return reflect.DeepEqual(toSet(a), toSet(b))
}

// originalSecurityInfo captures the pre-namespace global security requirements
// and the security scheme definitions they reference. This allows us to compare
// the effective security of two documents even after namespace prefixing has made
// the scheme names look different.
type originalSecurityInfo struct {
	requirements []*openapi.SecurityRequirement
	// schemes maps scheme name → serialized scheme definition (for equivalence checking).
	schemes map[string]string
}

// captureOriginalSecurity snapshots a document's security info before namespace application.
func captureOriginalSecurity(doc *openapi.OpenAPI) originalSecurityInfo {
	info := originalSecurityInfo{
		requirements: doc.Security,
		schemes:      make(map[string]string),
	}
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for name, scheme := range doc.Components.SecuritySchemes.All() {
			if scheme != nil && scheme.Object != nil {
				info.schemes[name] = fingerprintSecurityScheme(scheme.Object)
			}
		}
	}
	return info
}

// fingerprintSecurityScheme produces a comparable string from a security scheme's
// structural fields (ignoring description and extensions). This captures enough
// detail to distinguish schemes that share the same name but have different
// configurations (e.g. different tokenUrls, different types).
func fingerprintSecurityScheme(obj *openapi.SecurityScheme) string {
	inVal := ""
	if obj.In != nil {
		inVal = obj.In.String()
	}
	fp := fmt.Sprintf("type=%s|scheme=%s|bearerFormat=%s|name=%s|in=%s|openIdConnectUrl=%s",
		obj.Type,
		derefStr(obj.Scheme),
		derefStr(obj.BearerFormat),
		derefStr(obj.Name),
		inVal,
		derefStr(obj.OpenIdConnectUrl))

	// Include OAuth2 flow URLs to distinguish schemes with different token endpoints.
	if obj.Flows != nil {
		fp += "|flows="
		fp += fingerprintOAuthFlow("implicit", obj.Flows.Implicit)
		fp += fingerprintOAuthFlow("password", obj.Flows.Password)
		fp += fingerprintOAuthFlow("clientCredentials", obj.Flows.ClientCredentials)
		fp += fingerprintOAuthFlow("authorizationCode", obj.Flows.AuthorizationCode)
		fp += fingerprintOAuthFlow("deviceAuthorization", obj.Flows.DeviceAuthorization)
	}

	return fp
}

func fingerprintOAuthFlow(name string, flow *openapi.OAuthFlow) string {
	if flow == nil {
		return ""
	}
	return fmt.Sprintf("[%s:authUrl=%s,tokenUrl=%s]",
		name,
		derefStr(flow.AuthorizationURL),
		derefStr(flow.TokenURL))
}

// originalSecurityEqual checks whether two pre-namespace security infos represent
// the same effective security. It compares both the requirement names/scopes AND
// the underlying scheme definitions.
func originalSecurityEqual(a, b originalSecurityInfo) bool {
	if !securityRequirementsEqual(a.requirements, b.requirements) {
		return false
	}

	// Even if the requirement names match, the underlying scheme definitions
	// might differ (e.g. both use "bearerAuth" but one is http-bearer and
	// the other is apiKey). Check that all referenced schemes are equivalent.
	for _, req := range a.requirements {
		if req == nil {
			continue
		}
		for name := range req.All() {
			schemeA, okA := a.schemes[name]
			schemeB, okB := b.schemes[name]
			if okA != okB || schemeA != schemeB {
				return false
			}
		}
	}

	return true
}

// mergeSecurity handles merging of document-level security requirements.
// When two documents have different global security, the global security from each
// is inlined onto their respective operations (only those that don't already have
// inline security). The merged document ends up with no global security, and every
// operation carries its effective security explicitly.
//
// We compare the pre-namespace (original) security info rather than the
// post-namespace ones, because namespace prefixing makes equivalent schemes look
// different (e.g. svcA_bearerAuth vs svcB_bearerAuth).
//
// Returns (new mergedDoc security, new doc security, new merged original security info).
func mergeSecurity(mergedDoc, doc *openapi.OpenAPI, mergedOrig, docOrig originalSecurityInfo) ([]*openapi.SecurityRequirement, []*openapi.SecurityRequirement, originalSecurityInfo) {
	// Both nil — nothing to do.
	if mergedDoc.Security == nil && doc.Security == nil {
		return nil, nil, mergedOrig
	}

	// Compare the originals (pre-namespace) to determine if they're truly different.
	if originalSecurityEqual(mergedOrig, docOrig) {
		return mergedDoc.Security, doc.Security, mergedOrig
	}

	// Security differs — inline both sides' global security onto their operations
	// and clear global security so the merged document has none.
	setOperationSecurity(mergedDoc, mergedDoc.Security)
	setOperationSecurity(doc, doc.Security)

	// Both docs' global security is now cleared; operations carry it inline.
	return nil, nil, originalSecurityInfo{}
}

// setOperationSecurity inlines the given security requirements onto all operations
// in the document that don't already have explicit inline security.
// An operation with security == nil (not set) inherits global security, so it gets
// the global security inlined. An operation with security == [] (empty, meaning
// "no security") is left untouched.
func setOperationSecurity(doc *openapi.OpenAPI, security []*openapi.SecurityRequirement) {
	if doc.Paths == nil {
		return
	}

	for _, pathItem := range doc.Paths.All() {
		if pathItem.Object == nil {
			continue
		}

		for _, op := range pathItem.Object.All() {
			if op == nil {
				continue
			}
			// Only set security on operations that don't already have inline security.
			// security == nil means "not specified, inherit global".
			if op.Security == nil {
				// Copy the slice so operations don't share a backing array.
				copied := make([]*openapi.SecurityRequirement, len(security))
				copy(copied, security)
				op.Security = copied
			}
		}
	}
}
