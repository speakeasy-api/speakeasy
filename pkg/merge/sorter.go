package merge

import (
	"encoding/json"
	"fmt"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"sort"
)

func MapToOrderedMap[K constraints.Ordered, V any](m map[K]V) *orderedmap.OrderedMap[K, V] {
	om := orderedmap.New[K, V]()

	if m == nil {
		return nil
	}

	keys := GetSortedKeys(m)

	for _, k := range keys {
		om.Set(k, m[k])
	}

	return om
}

func GetSortedKeys[K constraints.Ordered, V any](m map[K]V) []K {
	keys := []K{}
	for k := range m {
		keys = append(keys, k)
	}

	slices.SortStableFunc(keys, func(i, j K) int {
		if i == j {
			return 0
		} else if i < j {
			return -1
		} else {
			return 1
		}
	})

	return keys
}

// LibOpenAPI currently doesn't deterministically sort external documents.
// This means top-level map items like pathItems/components are out of order post-merge
// This function sorts those.
func openapiSorter(input []byte) ([]byte, error) {
	var node yaml.Node
	var isJSON bool

	// Check if input is JSON
	if json.Unmarshal(input, &node) == nil {
		isJSON = true
	} else if err := yaml.Unmarshal(input, &node); err != nil {
		return nil, fmt.Errorf("input is neither valid JSON nor YAML: %w", err)
	}

	// Perform sorting on the YAML node
	sortOpenAPINode(&node)

	// Serialize back based on original format
	if isJSON {
		// Convert back to JSON
		intermediate, err := yaml.Marshal(&node)
		if err != nil {
			return nil, err
		}

		var jsonNode interface{}
		if err := yaml.Unmarshal(intermediate, &jsonNode); err != nil {
			return nil, err
		}

		return json.Marshal(jsonNode)
	} else {
		return yaml.Marshal(&node)
	}
}

// Very minimal sorter: sort `components.*.*` (recursively) and `paths.*`
func sortOpenAPINode(node *yaml.Node) {
	if node.Kind == yaml.DocumentNode {
		sortOpenAPINode(node.Content[0])
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if keyNode.Value == "components" && valNode.Kind == yaml.MappingNode {
				// json schemas always sorted alphabetically
				sortChild(valNode)
			}
			if keyNode.Value == "paths" {
				// rely on libopenapi order for anything below paths
				sortYAMLMap(valNode, false)
			}
		}
	}
}

func sortYAMLMap(node *yaml.Node, recurse bool) {
	if node.Kind != yaml.MappingNode {
		return
	}

	pairs := make([]keyValuePair, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		pairs = append(pairs, keyValuePair{Key: node.Content[i], Value: node.Content[i+1]})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Key.Value < pairs[j].Key.Value
	})

	var sortedContent []*yaml.Node
	for _, pair := range pairs {
		if recurse {
			sortChild(pair.Value)
		}
		sortedContent = append(sortedContent, pair.Key, pair.Value)
	}

	node.Content = sortedContent
}

func sortChild(value *yaml.Node) {
	if value.Kind == yaml.MappingNode {
		sortYAMLMap(value, true)
	} else if value.Kind == yaml.SequenceNode {
		for _, child := range value.Content {
			sortChild(child)
		}
	}
}

type keyValuePair struct {
	Key   *yaml.Node
	Value *yaml.Node
}
