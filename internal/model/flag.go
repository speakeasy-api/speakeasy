package model

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

type Flag interface {
	init(cmd *cobra.Command) error
	getName() string
	parseValue(v string) (interface{}, error)
}

type StringFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 string
}

func (f StringFlag) init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f StringFlag) getName() string {
	return f.Name
}

func (f StringFlag) parseValue(v string) (interface{}, error) {
	return v, nil
}

type BooleanFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 bool
}

func (f BooleanFlag) init(cmd *cobra.Command) error {
	cmd.Flags().BoolP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f BooleanFlag) getName() string {
	return f.Name
}

func (f BooleanFlag) parseValue(v string) (interface{}, error) {
	return strconv.ParseBool(v)
}

type IntFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 int
}

func (f IntFlag) init(cmd *cobra.Command) error {
	cmd.Flags().IntP(f.Name, f.Shorthand, f.DefaultValue, f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}

	return nil
}

func (f IntFlag) getName() string {
	return f.Name
}

func (f IntFlag) parseValue(v string) (interface{}, error) {
	return strconv.Atoi(v)
}

type MapFlag struct {
	Name, Shorthand, Description string
	Required, Hidden             bool
	DefaultValue                 map[string]string
}

func (f MapFlag) init(cmd *cobra.Command) error {
	cmd.Flags().StringP(f.Name, f.Shorthand, mapToString(f.DefaultValue), f.Description)
	if err := setRequiredAndHidden(cmd, f.Name, f.Required, f.Hidden); err != nil {
		return err
	}
	return nil
}

func (f MapFlag) getName() string {
	return f.Name
}

func (f MapFlag) parseValue(v string) (interface{}, error) {
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

func setRequiredAndHidden(cmd *cobra.Command, name string, required, hidden bool) error {
	if required {
		if err := cmd.MarkFlagRequired(name); err != nil {
			return err
		}
	}
	if hidden {
		if err := cmd.Flags().MarkHidden(name); err != nil {
			return err
		}
	}

	return nil
}

// Verify that the flag types implement the Flag interface
var _ = []Flag{
	&StringFlag{},
	&BooleanFlag{},
	&IntFlag{},
}
