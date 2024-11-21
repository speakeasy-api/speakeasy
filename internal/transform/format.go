package transform

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"gopkg.in/yaml.v3"
)

// NodeType represents the type of YAML node structure we're dealing with
type NodeType string

const (
	RootNode       NodeType = "RootNode"
	ComponentsNode NodeType = "ComponentsNode"
	PathNode       NodeType = "PathNode"
	InfoNode       NodeType = "InfoNode"
	OperationNode  NodeType = "OperationNode"
	SchemaNode     NodeType = "SchemaNode"
	ResponseNode   NodeType = "ResponseNode"
	SecurityNode   NodeType = "SecurityNode"
	UrlNode        NodeType = "UrlNode"
	TagNode        NodeType = "TagNode"
	UnknownNode    NodeType = "UnknownNode"
)

func (n NodeType) String() string {
	switch n {
	case RootNode:
		return "RootNode"
	case ComponentsNode:
		return "ComponentsNode"
	case PathNode:
		return "PathNode"
	case InfoNode:
		return "InfoNode"
	case OperationNode:
		return "OperationNode"
	case SchemaNode:
		return "SchemaNode"
	case ResponseNode:
		return "ResponseNode"
	case SecurityNode:
		return "SecurityNode"
	case UrlNode:
		return "UrlNode"
	case TagNode:
		return "TagNode"
	case UnknownNode:
		return "UnknownNode"
	default:
		return "UnknownNode"
	}
}

// Define the desired order of keys for different levels
var rootOrder = []string{
	"openapi",
	"info",
	"externalDocs",
	"security",
	"servers",
	"tags",
	"paths",
	"components",
}

var infoOrder = []string{
	"title",
	"version",
	"summary",
	"description",
	"termsOfService",
	"contact",
	"license",
}

var contactOrder = []string{
	"name",
	"url",
	"email",
}

var componentsOrder = []string{
	"schemas",
	"responses",
	"parameters",
	"examples",
	"requestBodies",
	"headers",
	"securitySchemes",
	"links",
	"callbacks",
}

var pathOrder = []string{
	"summary",
	"description",
	"servers",
	"parameters",
	"get",
	"post",
	"put",
	"delete",
	"patch",
	"head",
	"options",
	"trace",
}

var operationOrder = []string{
	"tags",
	"summary",
	"operationId",
	"description",
	"externalDocs",
	"parameters",
	"requestBody",
	"responses",
	"callbacks",
	"deprecated",
	"security",
	"servers",
}

var schemaOrder = []string{
	"name",
	"type",
	"title",
	"summary",
	"description",
	"in",
	"$ref",
	"format",
	"enum",
	"default",
	"multipleOf",
	"maximum",
	"exclusiveMaximum",
	"minimum",
	"exclusiveMinimum",
	"maxLength",
	"minLength",
	"pattern",
	"maxItems",
	"minItems",
	"uniqueItems",
	"maxProperties",
	"minProperties",
	"required",
	"properties",
	"items",
	"anyOf",
	"oneOf",
	"allOf",
	"not",
	"additionalProperties",
	"example",
	"examples",
	"deprecated",
}

var responseOrder = []string{
	"description",
	"headers",
	"content",
	"links",
}

var securityOrder = []string{
	"type",
	"description",
	"name",
	"in",
	"scheme",
	"bearerFormat",
	"flows",
	"openIdConnectUrl",
}

var parameterOrder = []string{
	"name",
	"in",
	"description",
	"required",
	"deprecated",
	"allowEmptyValue",
	"style",
	"explode",
	"allowReserved",
	"schema",
	"example",
	"examples",
	"content",
}

var requestBodyOrder = []string{
	"description",
	"content",
	"required",
}

var headerOrder = []string{
	"description",
	"required",
	"deprecated",
	"allowEmptyValue",
	"style",
	"explode",
	"schema",
	"example",
	"examples",
	"content",
}

var tagOrder = []string{
	"name",
	"description",
	"externalDocs",
}

var urlOrder = []string{
	"name",
	"identifier",
	"url",
	"description",
	"variables",
	"email",
}

var unknownOrder = []string{}

// Orders contains the order of keys for each node type
type Orders struct {
	rootOrder        []string
	infoOrder        []string
	contactOrder     []string
	componentsOrder  []string
	pathOrder        []string
	operationOrder   []string
	schemaOrder      []string
	responseOrder    []string
	securityOrder    []string
	parameterOrder   []string
	requestBodyOrder []string
	headerOrder      []string
	tagOrder         []string
	urlOrder         []string
	unknownOrder     []string
}

var orders = Orders{
	rootOrder:        rootOrder,
	infoOrder:        infoOrder,
	contactOrder:     contactOrder,
	componentsOrder:  componentsOrder,
	pathOrder:        pathOrder,
	operationOrder:   operationOrder,
	schemaOrder:      schemaOrder,
	responseOrder:    responseOrder,
	securityOrder:    securityOrder,
	parameterOrder:   parameterOrder,
	requestBodyOrder: requestBodyOrder,
	headerOrder:      headerOrder,
	tagOrder:         tagOrder,
	urlOrder:         urlOrder,
	unknownOrder:     unknownOrder,
}

// NodeTypeIdentifier contains sets of keys that identify different node types
var NodeTypeIdentifier = struct {
	httpMethods         []string
	tagKeys             []string
	schemaKeys          []string
	operationKeys       []string
	securityKeys        []string
	uniqueSecurityKeys  []string
	componentKeys       []string
	uniqueComponentKeys []string
	responseKeys        []string
}{
	httpMethods:         []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"},
	tagKeys:             []string{"name", "description", "externalDocs"},
	schemaKeys:          []string{"$ref", "type", "properties", "items", "required", "additionalProperties"},
	operationKeys:       []string{"operationId", "requestBody", "responses", "parameters", "tags"},
	securityKeys:        []string{"type", "scheme", "flows", "bearerFormat", "openIdConnectUrl"},
	uniqueSecurityKeys:  []string{"flows", "bearerFormat", "openIdConnectUrl"},
	componentKeys:       []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"},
	uniqueComponentKeys: []string{"schemas", "securitySchemes", "pathItems"},
	responseKeys:        []string{"content", "headers"},
}

// determineNodeType returns the appropriate NodeType based on the keys present
func determineNodeType(keys []string) NodeType {
	switch {
	// Root level identifiers
	case slices.Contains(keys, "openapi"):
		return RootNode

	// Operation objects
	case containsAny(keys, NodeTypeIdentifier.operationKeys) && !containsAny(keys, NodeTypeIdentifier.uniqueComponentKeys):
		return OperationNode

	// Components section
	case containsAny(keys, NodeTypeIdentifier.componentKeys):
		return ComponentsNode

	// URL-based nodes (servers, externalDocs, and license)
	case (slices.Contains(keys, "url")):
		return UrlNode

	// Path operations
	case containsAny(keys, NodeTypeIdentifier.httpMethods):
		return PathNode

	// Schema definitions
	case containsAny(keys, NodeTypeIdentifier.schemaKeys) && !containsAny(keys, NodeTypeIdentifier.uniqueSecurityKeys):
		return SchemaNode

	// Info objects
	case (slices.Contains(keys, "version") || slices.Contains(keys, "title")) &&
		!containsAny(keys, NodeTypeIdentifier.httpMethods):
		return InfoNode

	// Response objects
	case slices.Contains(keys, "description") &&
		containsAny(keys, NodeTypeIdentifier.responseKeys):
		return ResponseNode

	// Security objects
	case containsAny(keys, NodeTypeIdentifier.securityKeys):
		return SecurityNode

	// Tag objects
	case containsAny(keys, NodeTypeIdentifier.tagKeys):
		return TagNode

	default:
		return UnknownNode
	}
}

// containsAny returns true if slice contains any of the target values
func containsAny(slice, targets []string) bool {
	for _, target := range targets {
		if slices.Contains(slice, target) {
			return true
		}
	}
	return false
}

func FormatDocument(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: Format,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func FormatFromReader(ctx context.Context, schema io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	return transformer[interface{}]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: Format,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func Format(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], _ interface{}) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	root := model.Index.GetRootNode()

	if err := walkAndReorderNodes(ctx, root); err != nil {
		return doc, model, fmt.Errorf("failed to reorder nodes: %w", err)
	}

	updatedDoc, err := yaml.Marshal(root)
	if err != nil {
		return doc, model, fmt.Errorf("failed to marshal document: %w", err)
	}

	docNew, model, err := openapi.Load(updatedDoc, doc.GetConfiguration().BasePath)
	if err != nil {
		return doc, model, fmt.Errorf("failed to reload document: %w", err)
	}

	return *docNew, model, nil
}

func walkAndReorderNodes(ctx context.Context, node *yaml.Node) error {
	if node == nil || ctx.Err() != nil {
		return ctx.Err()
	}

	// Handle document nodes by processing their content
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			if err := walkAndReorderNodes(ctx, child); err != nil {
				return err
			}
		}
		return nil
	}

	// Only attempt to reorder mapping nodes
	if node.Kind == yaml.MappingNode {
		// Extract original keys
		originalKeys := make([]string, 0, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			originalKeys = append(originalKeys, node.Content[i].Value)
		}

		// Determine order and reorder node
		nodeType := determineNodeType(originalKeys)
		orderToUse := getOrderForType(nodeType, orders)
		if err := reorderYAMLNode(node, originalKeys, orderToUse); err != nil {
			return fmt.Errorf("failed to reorder node: %w", err)
		}
	}

	// Recursively process children for all node types
	for _, child := range node.Content {
		if err := walkAndReorderNodes(ctx, child); err != nil {
			return err
		}
	}

	return nil
}

func getOrderForType(nodeType NodeType, orders Orders) []string {
	switch nodeType {
	case RootNode:
		return orders.rootOrder
	case ComponentsNode:
		return orders.componentsOrder
	case PathNode:
		return orders.pathOrder
	case InfoNode:
		return orders.infoOrder
	case OperationNode:
		return orders.operationOrder
	case SchemaNode:
		return orders.schemaOrder
	case ResponseNode:
		return orders.responseOrder
	case SecurityNode:
		return orders.securityOrder
	case UrlNode:
		return orders.urlOrder
	case TagNode:
		return orders.tagOrder
	default:
		return orders.rootOrder
	}
}

func reorderYAMLNode(node *yaml.Node, originalKeys []string, order []string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("node is not a map")
	}

	// Create a map to hold key-value pairs by key
	kvMap := make(map[string]*yaml.Node)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		kvMap[keyNode.Value] = valueNode
	}

	// Clear the current Content slice
	node.Content = []*yaml.Node{}

	// First, append key-value pairs in the specified order
	for _, key := range order {
		if valueNode, ok := kvMap[key]; ok {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}, valueNode)
			// Remove from kvMap to track what's been processed
			delete(kvMap, key)
		}
	}

	// Then append any remaining keys that weren't in the order slice
	for _, key := range originalKeys {
		if valueNode, ok := kvMap[key]; ok {
			node.Content = append(node.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: key,
			}, valueNode)
		}
	}

	return nil
}
