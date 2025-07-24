package prompts

import (
	config "github.com/speakeasy-api/sdk-gen-config"
)

// Mapping of target form field names to their respective form field data.
type TargetFormFields map[string]*TargetFormField

// Adds a new target form field.
func (f TargetFormFields) Add(
	configField config.SDKGenConfigField,
	targetConfig config.LanguageConfig,
	targetName string,
	sdkClassName string,
	quickstart *Quickstart,
) (*TargetFormField, error) {
	targetFormField, err := NewTargetFormField(f, configField, targetConfig, targetName, sdkClassName, quickstart)

	if err != nil {
		return nil, err
	}

	if targetFormField == nil {
		return nil, nil
	}

	f[configField.Name] = targetFormField

	return targetFormField, nil
}

// Returns the target form field with the given name.
func (f TargetFormFields) GetField(fieldName string) *TargetFormField {
	field, ok := f[fieldName]

	if !ok {
		return nil
	}

	return field
}
