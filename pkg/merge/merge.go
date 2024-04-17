package merge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pb33f/libopenapi/utils"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func MergeOpenAPIDocuments(ctx context.Context, inFiles []string, outFile, defaultRuleset, workingDir string) error {
	inSchemas := make([][]byte, len(inFiles))

	// TODO at some point we prob want to support remote schemas
	for i, inFile := range inFiles {
		data, err := os.ReadFile(inFile)
		if err != nil {
			return err
		}

		if err := validate(ctx, inFile, data, defaultRuleset, workingDir); err != nil {
			log.From(ctx).Error(fmt.Sprintf("failed validating spec %s", inFile), zap.Error(err))
		}

		inSchemas[i] = data
	}

	mergedSchema, err := merge(inSchemas)
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

func validate(ctx context.Context, schemaPath string, schema []byte, defaultRuleset, workingDir string) error {
	logger := log.From(ctx)
	logger.Info(fmt.Sprintf("Validating OpenAPI spec %s...\n", schemaPath))

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	limits := &validation.OutputLimits{
		MaxWarns: 10,
	}

	vErrs, vWarns, _, _, err := validation.Validate(ctx, logger, schema, schemaPath, limits, false, defaultRuleset, workingDir)
	if err != nil {
		return err
	}

	for _, warn := range vWarns {
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range vErrs {
		prefixedLogger.Error("", zap.Error(err))
	}

	if len(vErrs) > 0 {
		status := "\nOpenAPI spec invalid ✖"
		return fmt.Errorf(status)
	}

	log.From(ctx).Success(fmt.Sprintf("Successfully validated %s", schemaPath))

	return nil
}

func merge(inSchemas [][]byte) ([]byte, error) {
	var mergedDoc *v3.Document
	var warnings []error

	for _, schema := range inSchemas {
		doc, err := loadOpenAPIDocument(schema)
		if err != nil {
			return nil, err
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
	rendered, err := mergedDoc.Render()
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		return rendered, multierror.Append(nil, warnings...)
	}

	return rendered, nil
}

// TODO better errors
func loadOpenAPIDocument(data []byte) (*v3.Document, error) {
	doc, err := libopenapi.NewDocumentWithConfiguration(data, &datamodel.DocumentConfiguration{
		AllowFileReferences:                 true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
	})
	if err != nil {
		return nil, err
	}

	if doc.GetSpecInfo().SpecType != utils.OpenApi3 {
		return nil, errors.New("only OpenAPI 3.x is supported")
	}

	model, buildErrs := doc.BuildV3Model()
	if len(buildErrs) > 0 {
		return nil, errors.Join(buildErrs...)
	}

	return &model.Model, nil
}

func MergeDocuments(mergedDoc, doc *v3.Document) (*v3.Document, []error) {
	mergedVersion, _ := version.NewSemver(mergedDoc.Version)
	docVersion, _ := version.NewSemver(doc.Version)
	errs := make([]error, 0)
	if mergedVersion == nil || docVersion != nil && docVersion.GreaterThan(mergedVersion) {
		mergedDoc.Version = doc.Version
	}

	if doc.Info != nil {
		mergedDoc.Info = doc.Info
	}

	if doc.Extensions != nil {
		var extErrors []error
		mergedDoc.Extensions, extErrors = mergeExtensions(mergedDoc.Extensions, doc.Extensions)
		for _, err := range extErrors {
			errs = append(errs, fmt.Errorf("%w in global extension", err))
		}
	}

	mergedServers, opServers := mergeServers(mergedDoc.Servers, doc.Servers, true)
	if len(opServers) > 0 {
		setOperationServers(mergedDoc, mergedDoc.Servers)
		setOperationServers(doc, opServers)
		mergedDoc.Servers = nil
	} else {
		mergedDoc.Servers = mergedServers
	}

	// TODO we might need to merge this such that the security from different docs are combined in an OR fashion
	if doc.Security != nil {
		mergedDoc.Security = doc.Security
	}

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

	if doc.Paths != nil {
		if mergedDoc.Paths == nil {
			mergedDoc.Paths = doc.Paths
		} else {
			var extensionErr []error
			mergedDoc.Paths.Extensions, extensionErr = mergeExtensions(mergedDoc.Paths.Extensions, doc.Paths.Extensions)
			errs = append(errs, extensionErr...)

			for pair := orderedmap.First(doc.Paths.PathItems); pair != nil; pair = pair.Next() {
				path := pair.Key()
				pathItem := pair.Value()
				if mergedPathItem, ok := mergedDoc.Paths.PathItems.Get(path); !ok {
					mergedDoc.Paths.PathItems.Set(path, pathItem)
				} else {
					pi, pathItemErrs := mergePathItems(mergedPathItem, pathItem)
					mergedDoc.Paths.PathItems.Set(path, pi)
					errs = append(errs, pathItemErrs...)
				}
			}
		}
	}

	if doc.Components != nil {
		if mergedDoc.Components == nil {
			mergedDoc.Components = doc.Components
		} else {
			var componentErrs []error
			mergedDoc.Components, componentErrs = mergeComponents(mergedDoc.Components, doc.Components)
			errs = append(errs, componentErrs...)
		}
	}

	if doc.Webhooks != nil {
		if mergedDoc.Webhooks == nil {
			mergedDoc.Webhooks = doc.Webhooks
		} else {
			for pair := orderedmap.First(doc.Webhooks); pair != nil; pair = pair.Next() {
				path := pair.Key()
				webhook := pair.Value()
				if _, ok := mergedDoc.Webhooks.Get(path); !ok {
					mergedDoc.Webhooks.Set(path, webhook)
				} else {
					pi, pathItemErrs := mergePathItems(mergedDoc.Webhooks.GetOrZero(path), webhook)
					mergedDoc.Webhooks.Set(path, pi)
					errs = append(errs, pathItemErrs...)
				}
			}
		}
	}

	if doc.ExternalDocs != nil {
		mergedDoc.ExternalDocs = doc.ExternalDocs
	}

	return mergedDoc, errs
}

func mergePathItems(mergedPathItem, pathItem *v3.PathItem) (*v3.PathItem, []error) {
	var errors []error
	if pathItem.Delete != nil {
		mergedPathItem.Delete = pathItem.Delete
	}

	if pathItem.Get != nil {
		mergedPathItem.Get = pathItem.Get
	}

	if pathItem.Head != nil {
		mergedPathItem.Head = pathItem.Head
	}

	if pathItem.Options != nil {
		mergedPathItem.Options = pathItem.Options
	}

	if pathItem.Patch != nil {
		mergedPathItem.Patch = pathItem.Patch
	}

	if pathItem.Post != nil {
		mergedPathItem.Post = pathItem.Post
	}

	if pathItem.Put != nil {
		mergedPathItem.Put = pathItem.Put
	}

	if pathItem.Trace != nil {
		mergedPathItem.Trace = pathItem.Trace
	}

	if pathItem.Summary != "" {
		mergedPathItem.Summary = pathItem.Summary
	}

	if pathItem.Description != "" {
		mergedPathItem.Description = pathItem.Description
	}

	mergedPathItem.Parameters = mergeParameters(mergedPathItem.Parameters, pathItem.Parameters)

	mergedPathItem.Servers, _ = mergeServers(mergedPathItem.Servers, pathItem.Servers, false)

	if pathItem.Extensions != nil {
		mergedPathItem.Extensions, errors = mergeExtensions(mergedPathItem.Extensions, pathItem.Extensions)
	}

	return mergedPathItem, errors
}

func mergeServers(mergedServers, servers []*v3.Server, global bool) ([]*v3.Server, []*v3.Server) {
	if len(mergedServers) == 0 {
		return servers, nil
	}

	if len(servers) > 0 {
		mergeable := !global

		if len(mergedServers) > 0 {
			for _, server := range servers {
				for _, mergedServer := range mergedServers {
					// We share common servers, so we can merge them
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

func mergeParameters(mergedParameters, parameters []*v3.Parameter) []*v3.Parameter {
	if len(mergedParameters) == 0 {
		return parameters
	}

	if len(parameters) > 0 {
		for _, parameter := range parameters {
			replaced := false

			for i, mergedParameter := range mergedParameters {
				if mergedParameter.Name == parameter.Name {
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

func mergeComponents(mergedComponents, components *v3.Components) (*v3.Components, []error) {
	errs := make([]error, 0)
	if components.Schemas != nil {
		if mergedComponents.Schemas == nil {
			mergedComponents.Schemas = components.Schemas
		} else {
			for pair := orderedmap.First(components.Schemas); pair != nil; pair = pair.Next() {
				name := pair.Key()
				schema := pair.Value()
				if err := isEquivalent(mergedComponents.Schemas.GetOrZero(name), schema); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.Schemas.Set(name, schema)
				mergedComponents.Schemas.GetOrZero(name).GoLow().GetKeyNode().Line = schema.GoLow().GetKeyNode().Line
			}
		}
	}

	if components.Responses != nil {
		if mergedComponents.Responses == nil {
			mergedComponents.Responses = components.Responses
		} else {
			for pair := orderedmap.First(components.Responses); pair != nil; pair = pair.Next() {
				name := pair.Key()
				response := pair.Value()
				if err := isEquivalent(mergedComponents.Responses.GetOrZero(name), response); err != nil {
					errs = append(errs, err)
				}
				mergedComponents.Responses.Set(name, response)
			}
		}
	}

	if components.Parameters != nil {
		if mergedComponents.Parameters == nil {
			mergedComponents.Parameters = components.Parameters
		} else {
			for pair := orderedmap.First(components.Parameters); pair != nil; pair = pair.Next() {
				name := pair.Key()
				parameter := pair.Value()
				if err := isEquivalent(mergedComponents.Parameters.GetOrZero(name), parameter); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.Parameters.Set(name, parameter)
			}
		}
	}

	if components.Examples != nil {
		if mergedComponents.Examples == nil {
			mergedComponents.Examples = components.Examples
		} else {
			for pair := orderedmap.First(components.Examples); pair != nil; pair = pair.Next() {
				name := pair.Key()
				example := pair.Value()
				if err := isEquivalent(mergedComponents.Examples.GetOrZero(name), example); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.Examples.Set(name, example)
			}
		}
	}

	if components.RequestBodies != nil {
		if mergedComponents.RequestBodies == nil {
			mergedComponents.RequestBodies = components.RequestBodies
		} else {
			for pair := orderedmap.First(components.RequestBodies); pair != nil; pair = pair.Next() {
				name := pair.Key()
				requestBody := pair.Value()
				if err := isEquivalent(mergedComponents.RequestBodies.GetOrZero(name), requestBody); err != nil {
					errs = append(errs, err)
				}
				mergedComponents.RequestBodies.Set(name, requestBody)
			}
		}
	}

	if components.Headers != nil {
		if mergedComponents.Headers == nil {
			mergedComponents.Headers = components.Headers
		} else {
			for pair := orderedmap.First(components.Headers); pair != nil; pair = pair.Next() {
				name := pair.Key()
				header := pair.Value()
				if err := isEquivalent(mergedComponents.Headers.GetOrZero(name), header); err != nil {
					errs = append(errs, err)
				}
				mergedComponents.Headers.Set(name, header)
			}
		}
	}

	if components.SecuritySchemes != nil {
		if mergedComponents.SecuritySchemes == nil {
			mergedComponents.SecuritySchemes = components.SecuritySchemes
		} else {
			for pair := orderedmap.First(components.SecuritySchemes); pair != nil; pair = pair.Next() {
				name := pair.Key()
				securityScheme := pair.Value()
				if err := isEquivalent(mergedComponents.SecuritySchemes.GetOrZero(name), securityScheme); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.SecuritySchemes.Set(name, securityScheme)
			}
		}
	}

	if components.Links != nil {
		if mergedComponents.Links == nil {
			mergedComponents.Links = components.Links
		} else {
			for pair := orderedmap.First(components.Links); pair != nil; pair = pair.Next() {
				name := pair.Key()
				link := pair.Value()
				if err := isEquivalent(mergedComponents.Links.GetOrZero(name), link); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.Links.Set(name, link)
			}
		}
	}

	if components.Callbacks != nil {
		if mergedComponents.Callbacks == nil {
			mergedComponents.Callbacks = components.Callbacks
		} else {
			for pair := orderedmap.First(components.Callbacks); pair != nil; pair = pair.Next() {
				name := pair.Key()
				callback := pair.Value()
				if err := isEquivalent(mergedComponents.Callbacks.GetOrZero(name), callback); err != nil {
					errs = append(errs, err)
				}

				mergedComponents.Callbacks.Set(name, callback)
			}
		}
	}

	if components.Extensions != nil {
		var extensionErrs []error
		mergedComponents.Extensions, extensionErrs = mergeExtensions(mergedComponents.Extensions, components.Extensions)
		errs = append(errs, extensionErrs...)
	}

	return mergedComponents, errs
}

type yamlComparable interface {
	MarshalYAMLInline() (interface{}, error)
	GoLow() interface {
		Hash() [32]byte
	}
}

type YAMLComparable interface {
	MarshalYAML() (interface{}, error)
}

func isEquivalent(a YAMLComparable, b YAMLComparable) error {
	if a == nil || (reflect.ValueOf(a).Kind() == reflect.Ptr && reflect.ValueOf(a).IsNil()) || b == nil || (reflect.ValueOf(b).Kind() == reflect.Ptr && reflect.ValueOf(b).IsNil()) {
		return nil
	}
	aInner, err := a.MarshalYAML()
	if err != nil {
		return fmt.Errorf("error marshalling %#v: %w", a, err)
	}

	bInner, err := b.MarshalYAML()
	if err != nil {
		return fmt.Errorf("error marshalling %#v: %w", a, err)
	}

	aNode := aInner.(*yaml.Node)
	bNode := bInner.(*yaml.Node)
	nodeOverlay, err := overlay.Compare("comparison between yaml nodes", "", aNode, *bNode)
	if err != nil {
		return fmt.Errorf("error comparing %#v and %#v: %w", a, b, err)
	}

	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}
	decodeA := yaml.NewEncoder(bufA)
	decodeB := yaml.NewEncoder(bufB)
	err = decodeA.Encode(aInner)
	if err != nil {
		return fmt.Errorf("failed to cast %#v to yaml.Node", aInner)
	}
	err = decodeB.Encode(bInner)
	if err != nil {
		return fmt.Errorf("failed to cast %#v to yaml.Node", bInner)
	}

	if len(nodeOverlay.Actions) > 0 {
		return fmt.Errorf("schemas are not equivalent: \nSchema 1 = %s\n\n Schema 2 = %s", bufA.String(), bufB.String())
	}

	return nil
}

func mergeExtensions(mergedExtensions, extensions *orderedmap.Map[string, *yaml.Node]) (*orderedmap.Map[string, *yaml.Node], []error) {
	if mergedExtensions == nil {
		return extensions, nil
	}
	errs := make([]error, 0)

	for pair := orderedmap.First(extensions); pair != nil; pair = pair.Next() {
		name := pair.Key()
		extYamlNode := pair.Value()

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

func setOperationServers(doc *v3.Document, opServers []*v3.Server) {
	if doc.Paths == nil {
		return
	}

	for pair := orderedmap.First(doc.Paths.PathItems); pair != nil; pair = pair.Next() {
		pathItem := pair.Value()
		ops := pathItem.GetOperations()

		for pair := orderedmap.First(ops); pair != nil; pair = pair.Next() {
			op := pair.Value()

			op.Servers, _ = mergeServers(op.Servers, opServers, false)
		}
	}
}
