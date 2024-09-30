package suggest

import (
	"context"
	"fmt"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy-core/yamlutil"
	"gopkg.in/yaml.v3"
	"slices"
	"strings"
)

type errorName string

const (
	BadRequest          errorName = "BadRequest"
	Unauthorized        errorName = "Unauthorized"
	NotFound            errorName = "NotFound"
	RateLimited         errorName = "RateLimited"
	InternalServerError errorName = "InternalServerError"
)

var codeOrder = []string{"400", "413", "414", "415", "401", "403", "407", "404", "429", "500"}
var codeToErrorName = map[string]errorName{
	"400": BadRequest,
	"401": Unauthorized,
	"403": Unauthorized,
	"404": NotFound,
	"407": Unauthorized,
	"413": BadRequest,
	"414": BadRequest,
	"415": BadRequest,
	"429": RateLimited,
	"500": InternalServerError,
}

var errorNameToDescription = map[errorName]string{
	BadRequest:          "Bad Request",
	Unauthorized:        "Unauthorized",
	NotFound:            "Not Found",
	RateLimited:         "Rate Limited",
	InternalServerError: "Internal Server Error",
}

func BuildErrorCodesOverlay(ctx context.Context, document v3.Document) (*overlay.Overlay, error) {
	// Track which operations are missing which response codes
	targetToMissingCodes := map[string][]string{}
	// Track if certain response codes are already defined elsewhere in the document so we don't duplicate them
	codeUsedComponents := map[string]map[string]int{}

	for op := range openapi.IterateOperations(document) {
		method, path, operation := op.Method, op.Path, op.Operation

		codes := orderedmap.New[string, *v3.Response]()
		if operation.Responses != nil && operation.Responses.Codes != nil {
			codes = operation.Responses.Codes
		}

		// TODO account for "complex" error responses that may not actually be returned by the endpoint
		for _, code := range codeOrder {
			if response, matchedCode := getResponseForCode(codes, code); response != nil {
				// If the response code is defined and is a ref, record the ref
				low := response.GoLow()
				if low.IsReference() {
					// The matchedCode will be, for example, 4XX if that's what's defined in the actual spec
					incrementCount(codeUsedComponents, matchedCode, low.GetReference())
				}
			} else {
				// Otherwise, record that the response code is missing
				target := overlay.NewTargetSelector(path, method) + `["responses"]`
				targetToMissingCodes[target] = append(targetToMissingCodes[target], code)
			}
		}
	}

	componentNames := getErrorSchemaNames(document)

	builder := yamlutil.NewBuilder(false)
	codeToExistingComponent := keepOnlyMostUsed(codeUsedComponents)

	// Create an update action for each target that has missing codes
	// If the code is already defined in the document, reuse the existing definition
	// Otherwise, record that we need to add the missing code
	var actions []overlay.Action
	var missingComponents []errorName

	// TODO: if 4XX is used elsewhere, use it when adding if possible
	for target, missingCodes := range targetToMissingCodes {
		var nodes []*yaml.Node
		for _, code := range missingCodes {
			ref, exists := getReferenceForCode(codeToExistingComponent, code, componentNames)
			nodes = append(nodes, builder.NewKeyNode(code), builder.NewMultinode("$ref", ref))
			if !exists {
				missingComponents = append(missingComponents, codeToErrorName[code])
			}
		}
		actions = append(actions, overlay.Action{
			Target: target,
			Update: *builder.NewMappingNode(nodes...),
		})
		// TODO: add modification extension
	}

	// Create an action to add the missing components
	var missingSchemaNodes []*yaml.Node
	var missingComponentNodes []*yaml.Node
	for _, missingComponent := range missingComponents {
		missingSchemaNodes = append(missingSchemaNodes, getSchemaNodes(missingComponent, componentNames)...)
		missingComponentNodes = append(missingComponentNodes, getComponentNodes(missingComponent, componentNames)...)
	}

	if len(missingSchemaNodes) > 0 {
		actions = append(actions, overlay.Action{
			Target: "$.components.schemas",
			Update: *builder.NewMappingNode(
				missingSchemaNodes...,
			),
		})
	}

	if len(missingComponentNodes) > 0 {
		actions = append(actions, overlay.Action{
			Target: "$.components.responses",
			Update: *builder.NewMappingNode(
				missingComponentNodes...,
			),
		})
	}

	return &overlay.Overlay{
		Version: "1.0.0",
		Info: overlay.Info{
			Title:   "Response Codes Overlay",
			Version: "0.0.0", // TODO: bump this version
		},
		Actions: actions,
	}, nil
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
func getReferenceForCode(codeToExistingComponent map[string]string, code string, componentNames map[errorName]errorComponentNames) (string, bool) {
	// If the exact code is defined, return it, otherwise check for a less specific code
	for _, c := range possibleCodes(code) {
		if ref, ok := codeToExistingComponent[c]; ok {
			return ref, true
		}
	}

	// Next, check if a code from that same "group" is already defined, e.g. 401 and 403 are both defined as Unauthorized
	for c, name := range codeToErrorName {
		if c == code {
			continue
		}
		if name == codeToErrorName[code] {
			if ref, ok := codeToExistingComponent[c]; ok {
				return ref, true
			}
		}
	}

	// If the code is not already defined, return our recommendation
	if errorName, ok := codeToErrorName[code]; ok {
		return fmt.Sprintf("#/components/responses/%s", componentNames[errorName].response), false
	}

	return "", false
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
func getSchemaNodes(errorName errorName, componentNames map[errorName]errorComponentNames) []*yaml.Node {
	// TODO account for schema/component name collisions
	errorSchemaName := componentNames[errorName].schema
	builder := yamlutil.NewBuilder(false)

	return []*yaml.Node{builder.NewKeyNode(errorSchemaName), builder.NewMultinode("type", "string")}
}

// Returns something like:
// InternalServerError:
//     description: Internal Server Error
//     content:
//         '*/*':
//             schema:
//                 $ref: '#/components/schemas/InternalServerErrorError'
func getComponentNodes(errorName errorName, componentNames map[errorName]errorComponentNames) []*yaml.Node {
	// TODO account for schema/component name collisions
	errorResponseName := componentNames[errorName].response
	errorSchemaName := componentNames[errorName].schema
	builder := yamlutil.NewBuilder(false)

	var nodes []*yaml.Node
	nodes = append(nodes, builder.NewNodeItem("description", errorNameToDescription[errorName])...)
	nodes = append(nodes, builder.NewKeyNode("content"))
	nodes = append(nodes, builder.NewNode("*/*", builder.NewNode("schema", builder.NewMultinode("$ref", "#/components/schemas/"+errorSchemaName))))

	return []*yaml.Node{builder.NewKeyNode(errorResponseName), builder.NewMappingNode(nodes...)}
}

func errorSchemaName(errorName errorName) string {
	if !strings.HasSuffix(string(errorName), "Error") {
		return string(errorName) + "Error"
	}
	return string(errorName)
}

type errorComponentNames struct {
	schema, response string
}

// Resolves naming conflicts between what we want to add and what is already in the document
func getErrorSchemaNames(document v3.Document) map[errorName]errorComponentNames {
	result := map[errorName]errorComponentNames{}

	var schemaNames []string
	for s := range openapi.IterateSchemas(document) {
		schemaNames = append(schemaNames, s.Name)
	}

	var responseNames []string
	for r := range openapi.IterateResponses(document) {
		responseNames = append(responseNames, r.Name)
	}

	for errorName := range errorNameToDescription {
		responseName := findUnusedName(string(errorName), responseNames)
		schemaName := findUnusedName(errorSchemaName(errorName), schemaNames)

		result[errorName] = errorComponentNames{
			schema:   schemaName,
			response: responseName,
		}

		// Add the new names to the lists to avoid reuse
		schemaNames = append(schemaNames, schemaName)
		responseNames = append(responseNames, responseName)
	}

	return result
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
