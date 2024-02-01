package prompts

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

// During quickstart we ask for a limited subset of configs per language
var quickstartScopedKeys = map[string][]string{
	"go": {
		"packageName",
	},
	"typescript": {
		"packageName",
	},
	"python": {
		"packageName",
	},
	"java": {
		"groupID",
		"artifactID",
	},
	"terraform": {},
	"docs": {
		"defaultLanguage",
	},
}

var ignoredKeys = []string{
	"version",
}

func PromptForTargetConfig(targetName string, target *workflow.Target, existingConfig *config.Configuration, isQuickstart bool) (*config.Configuration, error) {
	var output *config.Configuration
	if existingConfig != nil {
		output = existingConfig
	} else {
		var err error
		output, err = config.GetDefaultConfig(true, generate.GetLanguageConfigDefaults, map[string]bool{target.Target: true})
		if err != nil {
			return nil, errors.Wrapf(err, "error generating config for target %s of type %s", targetName, target.Target)
		}
	}

	var sdkClassName string
	if !isQuickstart && output.Generation.SDKClassName != "" {
		sdkClassName = output.Generation.SDKClassName
	}
	configFields := []huh.Field{
		huh.NewInput().
			Title("Name your SDK:").
			Placeholder("your users will access SDK methods with <sdk_name>.doThing()").
			Inline(true).
			Prompt(" ").
			Value(&sdkClassName),
	}

	var baseServerURL string
	if !isQuickstart && output.Generation.BaseServerURL != "" {
		baseServerURL = output.Generation.BaseServerURL
	}
	if !isQuickstart {
		configFields = append(configFields, huh.NewInput().
			Title("Provide a base server URL for your SDK to use:").
			Placeholder("You must do this if a server URL is not defined in your OpenAPI spec").
			Inline(true).
			Prompt(" ").
			Value(&baseServerURL))
	}

	var docsLanguages []string
	if target.Target == "docs" {
		if existingConfig != nil {
			if docsCfg, ok := existingConfig.Languages["docs"]; ok {
				if langs, ok := docsCfg.Cfg["docsLanguages"]; ok {
					for _, lang := range langs.([]interface{}) {
						docsLanguages = append(docsLanguages, lang.(string))
					}
				}
			}
		}

		configFields = append(configFields,
			huh.NewMultiSelect[string]().
				Title("Select your SDK Docs Languages:").
				Description("These languages will appear as options in your generated SDK Docs site.").
				Options(huh.NewOptions(generate.SupportedSDKDocsLanguages...)...).
				Value(&docsLanguages))
	}

	t, err := generate.GetTargetFromTargetString(target.Target)
	if err != nil {
		return nil, err
	}

	defaultConfigs, err := generate.GetLanguageConfigFields(t, true)
	if err != nil {
		return nil, err
	}

	languageForms, appliedKeys, err := languageSpecificForms(target.Target, output, defaultConfigs, isQuickstart)
	if err != nil {
		return nil, err
	}

	configFields = append(configFields, languageForms...)
	form := huh.NewForm(
		huh.NewGroup(
			configFields...,
		))
	if _, err := tea.NewProgram(charm.NewForm(form,
		fmt.Sprintf("Let's configure your %s target (%s)", target.Target, targetName),
		"This will create a gen.yaml config file that defines parameters for how your SDK is generated. \n"+
			"We will go through a few basic configurations here, but you can always modify this file directly in the future.")).
		Run(); err != nil {
		return nil, err
	}

	output.Generation.SDKClassName = sdkClassName
	output.Generation.BaseServerURL = baseServerURL
	if target.Target == "docs" {
		output.Languages["docs"].Cfg["docsLanguages"] = docsLanguages
	}

	saveLanguageConfigValues(target.Target, form, output, appliedKeys, defaultConfigs)

	return output, nil
}

func configBaseForm(quickstart *Quickstart) (*QuickstartState, error) {
	for key, target := range quickstart.WorkflowFile.Targets {
		output, err := PromptForTargetConfig(key, &target, nil, true)
		if err != nil {
			return nil, err
		}

		quickstart.LanguageConfigs[key] = output
	}

	var nextState QuickstartState = GithubWorkflowBase
	return &nextState, nil
}

func languageSpecificForms(language string, existingConfig *config.Configuration, configFields []config.SDKGenConfigField, isQuickstart bool) ([]huh.Field, []string, error) {
	langConfig := config.LanguageConfig{}
	if existingConfig != nil {
		if conf, ok := existingConfig.Languages[language]; ok {
			langConfig = conf
		}
	}

	fields := []huh.Field{}
	var appliedKeys []string
	for _, field := range configFields {
		if slices.Contains(ignoredKeys, field.Name) {
			continue
		}

		if valid, defaultValue, validateRegex, validateMessage := getValuesForField(field, langConfig); valid {
			if !isQuickstart {
				appliedKeys = append(appliedKeys, field.Name)
				fields = append(fields, addPromptForField(field.Name, defaultValue, validateRegex, validateMessage, field.Description, isQuickstart)...)
			} else if lang, ok := quickstartScopedKeys[language]; ok && slices.Contains(lang, field.Name) {
				appliedKeys = append(appliedKeys, field.Name)
				fields = append(fields, addPromptForField(field.Name, defaultValue, validateRegex, validateMessage, field.Description, isQuickstart)...)
			}
		}
	}

	return fields, appliedKeys, nil
}

func getValuesForField(field config.SDKGenConfigField, langConfig config.LanguageConfig) (bool, string, string, string) {
	defaultValue := ""
	if field.DefaultValue != nil {
		// We only support string and boolean fields at this particular moment, more to come.
		switch val := (*field.DefaultValue).(type) {
		case string:
			defaultValue = val
		case int:
			defaultValue = strconv.Itoa(val)
		case bool:
			defaultValue = strconv.FormatBool(val)
		default:
			return false, "", "", ""
		}
	}

	// Choose an existing default val if possible
	if value, ok := langConfig.Cfg[field.Name]; ok {
		switch val := value.(type) {
		case string:
			defaultValue = val
		case int:
			defaultValue = strconv.Itoa(val)
		case bool:
			defaultValue = strconv.FormatBool(val)
		}
	}

	validationRegex := ""
	if field.ValidationRegex != nil {
		validationRegex = *field.ValidationRegex
		validationRegex = strings.Replace(validationRegex, `\u002f`, `/`, -1)
	}

	validationMessage := ""
	if field.ValidationRegex != nil {
		validationMessage = *field.ValidationMessage
	}

	return true, defaultValue, validationRegex, validationMessage
}

func addPromptForField(key, defaultValue, validateRegex, validateMessage string, description *string, isQuickstart bool) []huh.Field {
	input := charm.NewInput().
		Key(key).
		Title(fmt.Sprintf("Provide a value for your %s config", key)).
		Validate(func(s string) error {
			if validateRegex != "" {
				r, err := regexp.Compile(validateRegex)
				if err != nil {
					return err
				}
				if !r.MatchString(s) {
					return errors.New(validateMessage)
				}
			}
			return nil
		})

	if description != nil {
		input = input.Description(*description + "\n").Inline(false).Prompt("")
	}

	if !isQuickstart && defaultValue != "" {
		input = input.Value(&defaultValue)
	} else {
		input = input.Placeholder(defaultValue).Suggestions([]string{defaultValue})
	}

	return []huh.Field{
		input,
	}
}

func saveLanguageConfigValues(language string, form *huh.Form, configuration *config.Configuration, appliedKeys []string, configFields []config.SDKGenConfigField) {
	for _, key := range appliedKeys {
		var field *config.SDKGenConfigField
		for _, f := range configFields {
			if f.Name == key {
				field = &f
				break
			}
		}
		if field != nil {
			// We need to map values back to their native type since the form only can produce a string
			if field.DefaultValue != nil {
				switch (*field.DefaultValue).(type) {
				case int:
					if transform, err := strconv.Atoi(form.GetString(key)); err == nil {
						configuration.Languages[language].Cfg[key] = transform
					}
				case bool:
					if transform, err := strconv.ParseBool(form.GetString(key)); err == nil {
						configuration.Languages[language].Cfg[key] = transform
					}
				case string:
					configuration.Languages[language].Cfg[key] = form.GetString(key)
				}
			} else {
				configuration.Languages[language].Cfg[key] = form.GetString(key)
			}
		}
	}
}
