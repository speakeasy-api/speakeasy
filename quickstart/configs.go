package quickstart

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/charm"
)

func configBaseForm(quickstart *Quickstart) (*State, error) {
	for key, target := range quickstart.WorkflowFile.Targets {
		output, err := config.GetDefaultConfig(true, getLanguageConfigDefaults, map[string]bool{target.Target: true})
		if err != nil {
			return nil, errors.Wrapf(err, "error generating config for target %s of type %s", key, target.Target)
		}

		var sdkClassName string
		configFields := []huh.Field{
			huh.NewInput().
				Title("Choose an sdkClassName for your target:").
				Inline(true).
				Prompt(" ").
				Value(&sdkClassName),
		}
		configFields = append(configFields, languageSpecificForms(target.Target)...)
		if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
			huh.NewGroup(
				configFields...,
			)),
			fmt.Sprintf("Let's setup a gen.yaml config for your target %s of type %s", key, target.Target),
			"A gen.yaml config defines parameters for how your SDK is generated. \n"+
				"We will go through a few basic configurations here, but you can always modify this file directly in the future.")).
			Run(); err != nil {
			return nil, err
		}

		output.Generation.SDKClassName = sdkClassName

		quickstart.LanguageConfigs[key] = output
	}

	var nextState State = Complete
	return &nextState, nil
}

// TODO: Export this from openapi-generation?
func getLanguageConfigDefaults(lang string, newSDK bool) (*config.LanguageConfig, error) {
	configFields, err := generate.GetLanguageConfigFields(lang, newSDK)
	if err != nil {
		return nil, err
	}

	var versionDefault string
	cfg := make(map[string]any, 0)

	for _, field := range configFields {
		if field.Name == "version" && field.DefaultValue != nil {
			versionDefault = (*field.DefaultValue).(string)
		} else {
			if field.DefaultValue != nil {
				cfg[field.Name] = *field.DefaultValue
			}
		}
	}

	return &config.LanguageConfig{
		Version: versionDefault,
		Cfg:     cfg,
	}, nil
}

// TODO: This is how we can add language specific forms for gen.yaml configs
func languageSpecificForms(language string) []huh.Field {
	switch language {
	case "typescript":
		return []huh.Field{}
	default:
		return []huh.Field{}
	}
}
