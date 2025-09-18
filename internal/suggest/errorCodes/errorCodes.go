package errorCodes

import (
	"context"
	"fmt"
	"slices"

	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/yml"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"gopkg.in/yaml.v3"
)

func BuildErrorCodesOverlay(ctx context.Context, schemaPath string) (*overlay.Overlay, error) {
	_, doc, err := schemas.LoadDocument(ctx, schemaPath)
	if err != nil {
		return nil, err
	}

	groups := initErrorGroups()
	groups.DeduplicateComponentNames(ctx, doc)

	builder := builder{document: doc, errorGroups: groups}
	o := builder.Build(ctx)
	return &o, nil
}

type builder struct {
	document    *openapi.OpenAPI
	errorGroups errorGroupSlice
}

type operation struct {
	operation *openapi.Operation
	path      string
	method    string
}

func (b *builder) Build(ctx context.Context) overlay.Overlay {
	// Track which operations are missing which response codes
	operationToMissingCodes := map[operation][]string{}
	// Track if certain response codes are already defined elsewhere in the document so we don't duplicate them
	codeUsedComponents := map[string]map[string]int{}

	for item := range openapi.Walk(ctx, b.document) {
		_ = item.Match(openapi.Matcher{
			Operation: func(o *openapi.Operation) error {
				method, path := openapi.ExtractMethodAndPath(item.Location)
				op := operation{operation: o, path: path, method: method}

				// TODO account for "complex" error responses that may not actually be returned by the endpoint
				for _, code := range b.errorGroups.AllCodes() {
					if response, matchedCode := getResponseForCode(o.GetResponses(), code); response != nil {
						// If the response code is defined and is a ref, record the ref
						if response.IsReference() {
							// The matchedCode will be, for example, 4XX if that's what's defined in the actual spec
							incrementCount(codeUsedComponents, matchedCode, response.GetReference().String())
						}
					} else {
						// Otherwise, record that the response code is missing
						operationToMissingCodes[op] = append(operationToMissingCodes[op], code)
					}
				}

				return nil
			},
		})
	}

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

			codeKey := yml.CreateStringNode(code)
			codeKey.Style = yaml.SingleQuotedStyle // Produces a warning otherwise

			nodes = append(nodes, codeKey, yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("$ref"), yml.CreateStringNode(ref)}))
			if !exists {
				missingGroup := b.errorGroups.FindCode(code)
				if !slices.ContainsFunc(missingComponents, func(g errorGroup) bool { return g.responseName == missingGroup.responseName }) {
					missingComponents = append(missingComponents, missingGroup)
				}
			}
		}

		target := overlay.NewTargetSelector(operation.path, operation.method) + `["responses"]`

		action := overlay.Action{
			Target: target,
			Update: *yml.CreateMapNode(ctx, nodes),
		}
		suggestions.AddModificationExtension(&action, &suggestions.ModificationExtension{
			Type:   suggestions.ModificationTypeErrorNames,
			Before: fmt.Sprintf("%s:\n\tcatch(SDKError) { ... }", operation.operation.GetOperationID()),
			After:  fmt.Sprintf("%s:\n\tcatch(Unauthorized) { ... }", operation.operation.GetOperationID()),
		})
		actions = append(actions, action)
	}

	// Create an action to add the missing components
	var missingSchemaNodes []*yaml.Node
	var missingComponentNodes []*yaml.Node
	for _, missingComponent := range missingComponents {
		missingSchemaNodes = append(missingSchemaNodes, getSchemaNodes(ctx, missingComponent)...)
		missingComponentNodes = append(missingComponentNodes, getComponentNodes(ctx, missingComponent)...)
	}

	if len(missingSchemaNodes) > 0 {
		schemasNode := *yml.CreateMapNode(ctx, missingSchemaNodes)

		// If components.schemas doesn't already exist, appending will fail silently
		// If components.schemas does exist, we don't want to overwrite it
		missingSchemasComponents := b.document.GetComponents().GetSchemas().Len() == 0
		if missingSchemasComponents {
			actions = append(actions, overlay.Action{
				Target: "$.components",
				Update: *yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("schemas"), &schemasNode}),
			})
		} else {
			actions = append(actions, overlay.Action{
				Target: "$.components.schemas",
				Update: schemasNode,
			})
		}
	}

	if len(missingComponentNodes) > 0 {
		responsesNode := *yml.CreateMapNode(ctx, missingComponentNodes)

		// If components.responses doesn't already exist, appending will fail silently
		// If components.responses does exist, we don't want to overwrite it
		missingResponseComponents := b.document.GetComponents().GetResponses().Len() == 0
		if missingResponseComponents {
			actions = append(actions, overlay.Action{
				Target: "$.components",
				Update: *yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("responses"), &responsesNode}),
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
func getResponseForCode(responses *openapi.Responses, code string) (*openapi.ReferencedResponse, string) {
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
// InternalServerError:
//
//	type: object
//	properties:
//	  message:
//	    type: string
//	additionalProperties: true
func getSchemaNodes(ctx context.Context, group errorGroup) []*yaml.Node {
	propertiesNode := yml.CreateMapNode(
		ctx,
		[]*yaml.Node{yml.CreateStringNode("message"), yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("type"), yml.CreateStringNode("string")})},
	)

	var nodes []*yaml.Node
	nodes = append(nodes, yml.CreateStringNode("type"), yml.CreateStringNode("object"))
	nodes = append(nodes, yml.CreateStringNode("x-speakeasy-suggested-error"), yml.CreateBoolNode(true))
	nodes = append(nodes, yml.CreateStringNode("properties"), propertiesNode)
	nodes = append(nodes, yml.CreateStringNode("additionalProperties"), yml.CreateBoolNode(true))

	return []*yaml.Node{
		yml.CreateStringNode(group.schemaName),
		yml.CreateMapNode(ctx, nodes),
	}
}

// Returns something like:
// InternalServerError:
//
//	description: Internal Server Error
//	content:
//	    application/json:
//	        schema:
//	            $ref: '#/components/schemas/InternalServerError'
func getComponentNodes(ctx context.Context, group errorGroup) []*yaml.Node {
	var nodes []*yaml.Node
	nodes = append(nodes, yml.CreateStringNode("description"), yml.CreateStringNode(group.description))

	schema := yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("$ref"), yml.CreateStringNode("#/components/schemas/" + group.schemaName)})
	content := yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("application/json"), yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("schema"), schema})})

	nodes = append(nodes, yml.CreateStringNode("content"), content)

	return []*yaml.Node{yml.CreateStringNode(group.responseName), yml.CreateMapNode(ctx, nodes)}
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
