package flag

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// MapFlag is a flag type for a map of strings
// Supports JSON input, as well as a simple key-value pair format
// Examples of valid input:
// --flag='{"key1":"value1","key2":"value2"}'
// --flag=key1:value1,key2:value2
type MapFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 map[string]string
}

func (f MapFlag) Init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, mapToString(f.DefaultValue), f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f MapFlag) GetName() string {
	return f.Name
}

func (f MapFlag) ParseValue(v string) (interface{}, error) {
	if v == "" || v == "null" {
		return make(map[string]string), nil
	}
	if v[0] == '{' {
		m := make(map[string]string)
		err := json.Unmarshal([]byte(v), &m)
		return m, err
	} else {
		return parseSimpleMap(v)
	}
}

func parseSimpleMap(v string) (map[string]string, error) {
	parts := strings.Split(v, ",")
	m := make(map[string]string)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part == "" {
			continue
		}

		kv := strings.Split(part, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid map format")
		}
		m[kv[0]] = kv[1]
	}

	return m, nil
}

func mapToString(m map[string]string) string {
	s, _ := json.Marshal(m)
	return string(s)
}
