package merge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	utils2 "github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/speakeasy-api/openapi/extensions"
	"github.com/speakeasy-api/openapi/jsonschema/oas3"
	"github.com/speakeasy-api/openapi/marshaller"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/overlay"
	"github.com/speakeasy-api/openapi/yml"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// MergeOpenAPIDocuments merges multiple OpenAPI documents into a single document.
// This is the legacy function that maintains backward compatibility.
func MergeOpenAPIDocuments(ctx context.Context, inFiles []string, outFile, defaultRuleset, workingDir string, skipGenerateLintReport bool) error {
	inputs := make([]MergeInput, len(inFiles))
	for i, inFile := range inFiles {
		inputs[i] = MergeInput{Path: inFile}
	}

	return MergeOpenAPIDocumentsWithNamespaces(ctx, inputs, outFile, MergeOptions{
		DefaultRuleset:         defaultRuleset,
		WorkingDir:             workingDir,
		SkipGenerateLintReport: skipGenerateLintReport,
		YAMLOutput:             utils2.HasYAMLExt(outFile),
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

		if err := validate(ctx, input.Path, data, opts.DefaultRuleset, opts.WorkingDir, opts.SkipGenerateLintReport); err != nil {
			log.From(ctx).Error(fmt.Sprintf("failed validating spec %s", input.Path), zap.Error(err))
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

func validate(ctx context.Context, schemaPath string, schema []byte, defaultRuleset, workingDir string, skipGenerateLintReport bool) error {
	logger := log.From(ctx)
	logger.Info(fmt.Sprintf("Validating OpenAPI spec %s...\n", schemaPath))

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	limits := &validation.OutputLimits{
		MaxWarns: 10,
	}

	res, err := validation.Validate(ctx, logger, schema, schemaPath, limits, false, defaultRuleset, workingDir, false, skipGenerateLintReport, "")
	if err != nil {
		return err
	}

	for _, warn := range res.Warnings {
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range res.Errors {
		prefixedLogger.Error("", zap.Error(err))
	}

	if len(res.Errors) > 0 {
		status := "\nOpenAPI spec invalid ✖"
		return errors.New(status)
	}

	log.From(ctx).Success(fmt.Sprintf("Successfully validated %s", schemaPath))

	return nil
}

func merge(ctx context.Context, inSchemas [][]byte, namespaces []string, yamlOut bool) ([]byte, error) {
	// Validate namespace consistency
	if err := validateNamespaceSlice(namespaces, len(inSchemas)); err != nil {
		return nil, err
	}

	var mergedDoc *openapi.OpenAPI
	var warnings []error

	for i, schema := range inSchemas {
		doc, err := loadOpenAPIDocument(ctx, schema)
		if err != nil {
			return nil, err
		}

		// Apply namespace if provided
		namespace := ""
		if i < len(namespaces) {
			namespace = namespaces[i]
		}

		if namespace != "" {
			applyNamespaceToSchemas(doc, namespace)

			if err := updateSchemaReferencesInDocument(ctx, doc, namespace); err != nil {
				return nil, fmt.Errorf("failed to update references for namespace %s: %w", namespace, err)
			}
		}

		if mergedDoc == nil {
			mergedDoc = doc
			continue
		}

		var errs []error
		mergedDoc, errs = MergeDocuments(mergedDoc, doc)
		warnings = append(warnings, errs...)
	}

	if mergedDoc == nil {
		return nil, errors.New("no documents to merge")
	}

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
func MergeDocuments(mergedDoc, doc *openapi.OpenAPI) (*openapi.OpenAPI, []error) {
	mergedVersion, _ := version.NewSemver(mergedDoc.OpenAPI)
	docVersion, _ := version.NewSemver(doc.OpenAPI)
	errs := make([]error, 0)

	if mergedVersion == nil || docVersion != nil && docVersion.GreaterThan(mergedVersion) {
		mergedDoc.OpenAPI = doc.OpenAPI
	}

	// Merge Info - take the newer one
	mergedDoc.Info = doc.Info

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

	// Merge Security
	if doc.Security != nil {
		mergedDoc.Security = doc.Security
	}

	// Merge Tags
	if doc.Tags != nil {
		if mergedDoc.Tags == nil {
			mergedDoc.Tags = doc.Tags
		} else {
			for _, tag := range doc.Tags {
				replaced := false
				for i, mergedTag := range mergedDoc.Tags {
					if mergedTag.Name == tag.Name {
						mergedDoc.Tags[i] = tag
						replaced = true
						break
					}
				}
				if !replaced {
					mergedDoc.Tags = append(mergedDoc.Tags, tag)
				}
			}
		}
	}

	// Merge Paths
	if doc.Paths != nil {
		if mergedDoc.Paths == nil {
			mergedDoc.Paths = doc.Paths
		} else {
			var extensionErr []error
			mergedDoc.Paths.Extensions, extensionErr = mergeExtensions(mergedDoc.Paths.Extensions, doc.Paths.Extensions)
			errs = append(errs, extensionErr...)

			for path, pathItem := range doc.Paths.All() {
				if mergedPathItem, ok := mergedDoc.Paths.Get(path); !ok {
					mergedDoc.Paths.Set(path, pathItem)
				} else {
					pi, pathItemErrs := mergePathItems(mergedPathItem, pathItem)
					mergedDoc.Paths.Set(path, pi)
					errs = append(errs, pathItemErrs...)
				}
			}
		}
	}

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

	return mergedDoc, errs
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

func mergePathItems(mergedPathItem, pathItem *openapi.ReferencedPathItem) (*openapi.ReferencedPathItem, []error) {
	return mergeReferencedPathItems(mergedPathItem, pathItem)
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

func setOperationServers(doc *openapi.OpenAPI, opServers []*openapi.Server) {
	if doc.Paths == nil {
		return
	}

	for _, pathItem := range doc.Paths.All() {
		if pathItem.Object == nil {
			continue
		}

		for _, op := range pathItem.Object.All() {
			if op != nil {
				op.Servers, _ = mergeServers(op.Servers, opServers, false)
			}
		}
	}
}
