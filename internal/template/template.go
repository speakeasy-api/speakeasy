package template

import (
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/speakeasy-api/easytemplate"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

func execute(templateLocation string, values interface{}) (string, error) {
	funcMap := map[string]any{
		"fromYaml": func(str string) map[string]interface{} {
			m := map[string]interface{}{}

			if err := yaml.Unmarshal([]byte(str), &m); err != nil {
				panic(err)
			}
			return m
		},
		"toYaml": func(v interface{}) string {
			data, err := yaml.Marshal(v)
			if err != nil {
				panic(err)
			}
			return strings.TrimSuffix(string(data), "\n")
		},
	}
	// Bring in the standard go functions that most people know/use (indent, nindent, etc)
	for k, v := range sprig.TxtFuncMap() {
		funcMap[k] = v
	}
	e := easytemplate.New(
		easytemplate.WithSearchLocations([]string{filepath.Dir(templateLocation)}),
		easytemplate.WithTemplateFuncs(funcMap),
		easytemplate.WithWriteFunc(func(outFile string, data []byte) error {
			return fmt.Errorf("write function not available")
		}),
	)

	created, err := e.RunTemplateString(templateLocation, values)
	if err != nil {
		return "", fmt.Errorf("Failed to execute template: %w", err)
	}
	return created, nil
}

func Execute(templateFileLocation string, valuesFileLocation string, outputFileLocation string) error {
	valuesString, err := os.ReadFile(valuesFileLocation)
	if err != nil {
		return fmt.Errorf("Failed to read values file: %w", err)
	}
	templateFileAbsPath, err := filepath.Abs(templateFileLocation)
	if err != nil {
		return fmt.Errorf("Failed to get absolute path of template file: %w", err)
	}
	values := make(map[string]interface{})
	if err = yaml.Unmarshal(valuesString, &values); err != nil {
		return fmt.Errorf("Failed to unmarshal values file: %w", err)
	}

	templated, err := execute(templateFileAbsPath, values)
	if err != nil {
		return fmt.Errorf("Failed to execute template: %w", err)
	}
	os.WriteFile(outputFileLocation, []byte(templated), 0644)
	return nil
}
