package suggestions

import (
	"encoding/json"
	"fmt"
	"github.com/iancoleman/orderedmap"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"reflect"
	"strings"
)

func convertJsonToYaml(file []byte) ([]byte, error) {
	data := orderedmap.New()
	err := json.Unmarshal(file, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	yamlMapSlice := jsonToYaml(*data)

	yamlFile, err := yaml.Marshal(&yamlMapSlice)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %v", err)
	}

	return yamlFile, nil
}

func detectFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".yaml", ".yml":
		return "text/yaml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func convertYamlToJson(file []byte) ([]byte, error) {
	yamlFile := yaml.MapSlice{}
	err := yaml.Unmarshal(file, &yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	jsonOrderedMap := yamlToJson(yamlFile)

	jsonFile, err := json.MarshalIndent(jsonOrderedMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return jsonFile, nil
}

func yamlToJson(yamlFile yaml.MapSlice) *orderedmap.OrderedMap {
	orderedJson := orderedmap.New()
	for _, item := range yamlFile {
		orderedJson.Set(item.Key.(string), recurseYamlToJson(item.Value))
	}

	return orderedJson
}

func recurseYamlToJson(v interface{}) interface{} {
	switch reflect.TypeOf(v) {
	case reflect.TypeOf(yaml.MapSlice{}):
		return yamlToJson(v.(yaml.MapSlice))
	case reflect.TypeOf([]interface{}{}):
		vals := make([]interface{}, 0)
		for _, v := range v.([]interface{}) {
			vals = append(vals, recurseYamlToJson(v))
		}

		return vals
	default:
		return v
	}
}

func jsonToYaml(jsonFile orderedmap.OrderedMap) yaml.MapSlice {
	yamlFile := yaml.MapSlice{}
	for _, key := range jsonFile.Keys() {
		value, _ := jsonFile.Get(key)
		yamlFile = append(yamlFile, yaml.MapItem{
			Key:   key,
			Value: recurseJsonToYaml(value),
		})
	}

	return yamlFile
}

func recurseJsonToYaml(v interface{}) interface{} {
	switch reflect.TypeOf(v) {
	case reflect.TypeOf(orderedmap.OrderedMap{}):
		return jsonToYaml(v.(orderedmap.OrderedMap))
	case reflect.TypeOf([]interface{}{}):
		vals := make([]interface{}, 0)
		for _, v := range v.([]interface{}) {
			vals = append(vals, recurseJsonToYaml(v))
		}

		return vals
	default:
		return v
	}
}

// matchOrder attempts to match the order of the keys in map 'm' to the order of the keys in 'toMatch'
// Keys that are not in 'toMatch' are appended to the end of the map
func matchOrder(m *orderedmap.OrderedMap, toMatch *orderedmap.OrderedMap) {
	f := func(keys []string) {
		newKeys := make([]string, len(keys))

		cur := 0
		// Insert all the original keys in their original order
		for _, k := range toMatch.Keys() {
			// Only add if the key is still present
			if slices.Contains(keys, k) {
				newKeys[cur] = k
				cur++
			}
		}

		// Put everything that wasn't in the original map at the end
		for _, k := range keys {
			if !slices.Contains(toMatch.Keys(), k) {
				newKeys[cur] = k
				cur++
			}
		}

		copy(keys, newKeys)
	}

	m.SortKeys(f)

	for k, v := range m.Values() {
		match, _ := toMatch.Get(k)
		matchOrderRecurse(v, match)
	}
}

func matchOrderRecurse(v interface{}, toMatch interface{}) {
	if v == nil || toMatch == nil {
		return
	}

	if reflect.TypeOf(v) == reflect.TypeOf([]interface{}{}) {
		for i, vSub := range v.([]interface{}) {
			if i < len(toMatch.([]interface{})) {
				matchOrderRecurse(vSub, toMatch.([]interface{})[i])
			}
		}
	} else {
		vOrdered, isOrderedMap := v.(orderedmap.OrderedMap)
		matchVOrderedMap, matchIsOrderedMap := toMatch.(orderedmap.OrderedMap)
		if isOrderedMap && matchIsOrderedMap {
			matchOrder(&vOrdered, &matchVOrderedMap)
		}
	}
}
