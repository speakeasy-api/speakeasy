package validation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type FieldValidation = func(v any, template string) error
type CustomTargetValidations = map[string]FieldValidation
type CustomValidations = map[string]CustomTargetValidations

var customValidations = CustomValidations{
	"python": {
		"additionalDependencies": pythonAdditionalDependenciesValidation,
	},
}

func getValidation(target, fieldName string) FieldValidation {
	if v, ok := customValidations[target]; ok {
		if f, ok := v[fieldName]; ok {
			return f
		}
	}

	return nil
}

type pythonAdditionalDependencies struct {
	Dependencies      map[string]string
	ExtraDependencies map[string]map[string]string
}

var validPrefixes = []string{"==", ">=", ">", "~=", "<", "<=", "!=", "==="}

func pythonAdditionalDependenciesValidation(v any, template string) error {
	j, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var ad pythonAdditionalDependencies
	if err := json.Unmarshal(j, &ad); err != nil {
		return err
	}

	// This check does not apply to the pythonv2 template
	if (ad.Dependencies == nil || ad.ExtraDependencies == nil) && template == "python" {
		return fmt.Errorf("either dependencies or extraDependencies must be provided, or the entire field should be omitted")
	}

	validateDepMap := func(m map[string]string) error {
		for k, v := range m {
			if v == "" {
				return fmt.Errorf("dependency %s must have a version", k)
			}

			validPrefix := false
			for _, prefix := range validPrefixes {
				if strings.HasPrefix(v, prefix) {
					validPrefix = true
				}
			}
			if !validPrefix {
				return fmt.Errorf("dependency %s must start with one of: %s", k, strings.Join(validPrefixes, ", "))
			}
		}
		return nil
	}

	if err := validateDepMap(ad.Dependencies); err != nil {
		return err
	}
	for k, v := range ad.ExtraDependencies {
		if err := validateDepMap(v); err != nil {
			return fmt.Errorf("extra dependency %s is invalid: %w", k, err)
		}
	}

	return nil
}
