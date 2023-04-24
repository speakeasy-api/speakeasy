package merge

import (
	"errors"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/utils"
)

func MergeOpenAPIDocuments(inFiles []string, outFile string) error {
	inSchemas := make([][]byte, len(inFiles))

	for i, inFile := range inFiles {
		data, err := os.ReadFile(inFile)
		if err != nil {
			return err
		}

		inSchemas[i] = data
	}

	mergedSchema, err := merge(inSchemas)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outFile, mergedSchema, 0o644); err != nil {
		return err
	}

	return nil
}

func merge(inSchemas [][]byte) ([]byte, error) {
	var mergedDoc *v3.Document

	for _, schema := range inSchemas {
		doc, err := loadOpenAPIDocument(schema)
		if err != nil {
			return nil, err
		}

		if mergedDoc == nil {
			mergedDoc = doc
			continue
		}

		mergedDoc, err = mergeDocuments(mergedDoc, doc)
		if err != nil {
			return nil, err
		}
	}

	if mergedDoc == nil {
		return nil, errors.New("no documents to merge")
	}

	return mergedDoc.Render()
}

// TODO better errors
func loadOpenAPIDocument(data []byte) (*v3.Document, error) {
	doc, err := libopenapi.NewDocument(data)
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

func mergeDocuments(mergedDoc, doc *v3.Document) (*v3.Document, error) {
	mergedVersion, _ := version.NewSemver(mergedDoc.Version)
	docVersion, _ := version.NewSemver(doc.Version)
	if mergedVersion == nil || docVersion != nil && docVersion.GreaterThan(mergedVersion) {
		mergedDoc.Version = doc.Version
	}

	if doc.Info != nil {
		mergedDoc.Info = doc.Info
	}

	if doc.Extensions != nil {
		mergedDoc.Extensions = mergeExtensions(mergedDoc.Extensions, doc.Extensions)
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
			mergedDoc.Paths.Extensions = mergeExtensions(mergedDoc.Paths.Extensions, doc.Paths.Extensions)

			for path, pathItem := range doc.Paths.PathItems {
				if mergedPathItem, ok := mergedDoc.Paths.PathItems[path]; !ok {
					mergedDoc.Paths.PathItems[path] = pathItem
				} else {
					mergedDoc.Paths.PathItems[path] = mergePathItems(mergedPathItem, pathItem)
				}
			}
		}
	}

	if doc.Components != nil {
		if mergedDoc.Components == nil {
			mergedDoc.Components = doc.Components
		} else {
			mergedDoc.Components = mergeComponents(mergedDoc.Components, doc.Components)
		}
	}

	if doc.Webhooks != nil {
		if mergedDoc.Webhooks == nil {
			mergedDoc.Webhooks = doc.Webhooks
		} else {
			for path, webhook := range doc.Webhooks {
				if _, ok := mergedDoc.Webhooks[path]; !ok {
					mergedDoc.Webhooks[path] = webhook
				} else {
					mergedDoc.Webhooks[path] = mergePathItems(mergedDoc.Webhooks[path], webhook)
				}
			}
		}
	}

	if doc.ExternalDocs != nil {
		mergedDoc.ExternalDocs = doc.ExternalDocs
	}

	return mergedDoc, nil
}

func mergePathItems(mergedPathItem, pathItem *v3.PathItem) *v3.PathItem {
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
		mergedPathItem.Extensions = mergeExtensions(mergedPathItem.Extensions, pathItem.Extensions)
	}

	return mergedPathItem
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

func mergeComponents(mergedComponents, components *v3.Components) *v3.Components {
	if components.Schemas != nil {
		if mergedComponents.Schemas == nil {
			mergedComponents.Schemas = components.Schemas
		} else {
			for name, schema := range components.Schemas {
				mergedComponents.Schemas[name] = schema
			}
		}
	}

	if components.Responses != nil {
		if mergedComponents.Responses == nil {
			mergedComponents.Responses = components.Responses
		} else {
			for name, response := range components.Responses {
				mergedComponents.Responses[name] = response
			}
		}
	}

	if components.Parameters != nil {
		if mergedComponents.Parameters == nil {
			mergedComponents.Parameters = components.Parameters
		} else {
			for name, parameter := range components.Parameters {
				mergedComponents.Parameters[name] = parameter
			}
		}
	}

	if components.Examples != nil {
		if mergedComponents.Examples == nil {
			mergedComponents.Examples = components.Examples
		} else {
			for name, example := range components.Examples {
				mergedComponents.Examples[name] = example
			}
		}
	}

	if components.RequestBodies != nil {
		if mergedComponents.RequestBodies == nil {
			mergedComponents.RequestBodies = components.RequestBodies
		} else {
			for name, requestBody := range components.RequestBodies {
				mergedComponents.RequestBodies[name] = requestBody
			}
		}
	}

	if components.Headers != nil {
		if mergedComponents.Headers == nil {
			mergedComponents.Headers = components.Headers
		} else {
			for name, header := range components.Headers {
				mergedComponents.Headers[name] = header
			}
		}
	}

	if components.SecuritySchemes != nil {
		if mergedComponents.SecuritySchemes == nil {
			mergedComponents.SecuritySchemes = components.SecuritySchemes
		} else {
			for name, securityScheme := range components.SecuritySchemes {
				mergedComponents.SecuritySchemes[name] = securityScheme
			}
		}
	}

	if components.Links != nil {
		if mergedComponents.Links == nil {
			mergedComponents.Links = components.Links
		} else {
			for name, link := range components.Links {
				mergedComponents.Links[name] = link
			}
		}
	}

	if components.Callbacks != nil {
		if mergedComponents.Callbacks == nil {
			mergedComponents.Callbacks = components.Callbacks
		} else {
			for name, callback := range components.Callbacks {
				mergedComponents.Callbacks[name] = callback
			}
		}
	}

	if components.Extensions != nil {
		mergedComponents.Extensions = mergeExtensions(mergedComponents.Extensions, components.Extensions)
	}

	return mergedComponents
}

func mergeExtensions(mergedExtensions, extensions map[string]interface{}) map[string]interface{} {
	if mergedExtensions == nil {
		return extensions
	}

	for name, extension := range extensions {
		mergedExtensions[name] = extension
	}

	return mergedExtensions
}

func setOperationServers(doc *v3.Document, opServers []*v3.Server) {
	if doc.Paths == nil {
		return
	}

	for _, pathItem := range doc.Paths.PathItems {
		for _, op := range pathItem.GetOperations() {
			op.Servers, _ = mergeServers(op.Servers, opServers, false)
		}
	}
}
