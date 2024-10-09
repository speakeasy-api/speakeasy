package errorCodes

import (
	"context"
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy-core/yamlutil"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"gopkg.in/yaml.v3"
	"slices"
)

func BuildErrorCodesOverlay(ctx context.Context, schemaPath string) (*overlay.Overlay, error) {
	_, _, model, err := schemas.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}

	groups := initErrorGroups()
	groups.DeduplicateComponentNames(model.Model)

	builder := builder{document: model.Model, errorGroups: groups}
	o := builder.Build()
	return &o, nil
}

type builder struct {
	document    v3.Document
	errorGroups errorGroupSlice
}

func (b *builder) Build() overlay.Overlay {
	// Track which operations are missing which response codes
	operationToMissingCodes := map[openapi.OperationElem][]string{}
	// Track if certain response codes are already defined elsewhere in the document so we don't duplicate them
	codeUsedComponents := map[string]map[string]int{}

	for op := range openapi.IterateOperations(b.document) {
		operation := op.Operation

		codes := orderedmap.New[string, *v3.Response]()
		if operation.Responses != nil && operation.Responses.Codes != nil {
			codes = operation.Responses.Codes
		}

		// TODO account for "complex" error responses that may not actually be returned by the endpoint
		for _, code := range b.errorGroups.AllCodes() {
			if response, matchedCode := getResponseForCode(codes, code); response != nil {
				// If the response code is defined and is a ref, record the ref
				low := response.GoLow()
				if low.IsReference() {
					// The matchedCode will be, for example, 4XX if that's what's defined in the actual spec
					incrementCount(codeUsedComponents, matchedCode, low.GetReference())
				}
			} else {
				// Otherwise, record that the response code is missing
				operationToMissingCodes[op] = append(operationToMissingCodes[op], code)
			}
		}
	}

	builder := yamlutil.NewBuilder(false)
	codeToExistingComponent := keepOnlyMostUsed(codeUsedComponents)

	// Create an update action for each target that has missing codes
	// If the code is already defined in the document, reuse the existing definition
	// Otherwise, record that we need to add the missing code
	var actions []overlay.Action
	var missingComponents []errorGroup

	// TODO: if 4XX is used elsewhere, use it when adding if possible
	for operation, missingCodes := range operationToMissingCodes {
		var nodes []*yaml.Node
		for _, code := range missingCodes {
			ref, exists := b.getReferenceForCode(codeToExistingComponent, code)
			nodes = append(nodes, builder.NewKeyNode(code), builder.NewMultinode("$ref", ref))
			if !exists {
				missingGroup := b.errorGroups.FindCode(code)
				if !slices.ContainsFunc(missingComponents, func(g errorGroup) bool { return g.responseName == missingGroup.responseName }) {
					missingComponents = append(missingComponents, missingGroup)
				}
			}
		}

		target := overlay.NewTargetSelector(operation.Path, operation.Method) + `["responses"]`

		action := overlay.Action{
			Target: target,
			Update: *builder.NewMappingNode(nodes...),
		}
		suggestions.AddModificationExtension(&action, &suggestions.ModificationExtension{
			Type:   suggestions.ModificationTypeErrorNames,
			Before: fmt.Sprintf("%s:\n\tcatch(SDKError) { ... }", operation.Operation.OperationId),
			After:  fmt.Sprintf("%s:\n\tcatch(Unauthorized) { ... }", operation.Operation.OperationId),
		})
		actions = append(actions, action)
	}

	// Create an action to add the missing components
	var missingSchemaNodes []*yaml.Node
	var missingComponentNodes []*yaml.Node
	for _, missingComponent := range missingComponents {
		missingSchemaNodes = append(missingSchemaNodes, getSchemaNodes(missingComponent)...)
		missingComponentNodes = append(missingComponentNodes, getComponentNodes(missingComponent)...)
	}

	if len(missingSchemaNodes) > 0 {
		schemasNode := *builder.NewMappingNode(missingSchemaNodes...)

		// If components.schemas doesn't already exist, appending will fail silently
		// If components.schemas does exist, we don't want to overwrite it
		missingSchemasComponents := b.document.Components == nil || b.document.Components.Schemas == nil || b.document.Components.Schemas.IsZero()
		if missingSchemasComponents {
			actions = append(actions, overlay.Action{
				Target: "$.components",
				Update: *builder.NewNode("schemas", &schemasNode),
			})
		} else {
			actions = append(actions, overlay.Action{
				Target: "$.components.schemas",
				Update: schemasNode,
			})
		}
	}

	if len(missingComponentNodes) > 0 {
		responsesNode := *builder.NewMappingNode(missingComponentNodes...)

		// If components.responses doesn't already exist, appending will fail silently
		// If components.responses does exist, we don't want to overwrite it
		missingResponseComponents := b.document.Components == nil || b.document.Components.Responses == nil || b.document.Components.Responses.IsZero()
		if missingResponseComponents {
			actions = append(actions, overlay.Action{
				Target: "$.components",
				Update: *builder.NewNode("responses", &responsesNode),
			})
		} else {
			actions = append(actions, overlay.Action{
				Target: "$.components.responses",
				Update: responsesNode,
			})
		}
	}

	return overlay.Overlay{
		Version: "1.0.0",
		Info: overlay.Info{
			Title:   "Response Codes Overlay",
			Version: "0.0.0", // TODO: bump this version
		},
		Actions: actions,
	}
}

func incrementCount(m map[string]map[string]int, code, path string) {
	if cur, ok := m[code]; !ok {
		m[code] = map[string]int{path: 1}
	} else {
		cur[path] += 1
	}
}

// returns the response and the actual code that was matched
func getResponseForCode(responses *orderedmap.Map[string, *v3.Response], code string) (*v3.Response, string) {
	for _, c := range possibleCodes(code) {
		if v, ok := responses.Get(c); ok {
			return v, c
		}
	}

	return nil, ""
}

// returns the reference to the component that defines the code, and whether the component already exists in the document
func (b *builder) getReferenceForCode(codeToExistingComponent map[string]string, code string) (string, bool) {
	// If the exact code is defined, return it, otherwise check for a less specific code
	for _, c := range possibleCodes(code) {
		if ref, ok := codeToExistingComponent[c]; ok {
			return ref, true
		}
	}

	// Next, check if a code from that same "group" is already defined, e.g. 401 and 403 are both defined as Unauthorized
	for _, c := range b.errorGroups.FindCode(code).codes {
		if c == code {
			continue
		}
		if ref, ok := codeToExistingComponent[c]; ok {
			return ref, true
		}
	}

	// If the code is not already defined, return our recommendation
	return fmt.Sprintf("#/components/responses/%s", b.errorGroups.FindCode(code).responseName), false
}

func possibleCodes(code string) []string {
	return []string{
		code,
		string(code[0]) + "XX",
		string(code[0]) + "xx",
	}
}

func keepOnlyMostUsed(codeUsedComponents map[string]map[string]int) map[string]string {
	result := map[string]string{}
	for code, refs := range codeUsedComponents {
		var maxCount int
		var maxKey string
		for key, count := range refs {
			if count > maxCount {
				maxCount = count
				maxKey = key
			}
		}
		if maxKey != "" {
			result[code] = maxKey
		}
	}
	return result
}

// Returns something like:
// InternalServerErrorError:
//     type: string
func getSchemaNodes(group errorGroup) []*yaml.Node {
	builder := yamlutil.NewBuilder(false)

	return []*yaml.Node{builder.NewKeyNode(group.schemaName), builder.NewMultinode("type", "string")}
}

// Returns something like:
// InternalServerError:
//     description: Internal Server Error
//     content:
//         '*/*':
//             schema:
//                 $ref: '#/components/schemas/InternalServerErrorError'
func getComponentNodes(group errorGroup) []*yaml.Node {
	builder := yamlutil.NewBuilder(false)

	var nodes []*yaml.Node
	nodes = append(nodes, builder.NewNodeItem("description", group.description)...)
	nodes = append(nodes, builder.NewKeyNode("content"))
	nodes = append(nodes, builder.NewNode("*/*", builder.NewNode("schema", builder.NewMultinode("$ref", "#/components/schemas/"+group.schemaName))))

	return []*yaml.Node{builder.NewKeyNode(group.responseName), builder.NewMappingNode(nodes...)}
}

func findUnusedName(baseName string, usedNames []string) string {
	name := baseName
	counter := 1
	for slices.Contains(usedNames, name) {
		name = fmt.Sprintf("%s%d", baseName, counter)
		counter++
	}
	return name
}
