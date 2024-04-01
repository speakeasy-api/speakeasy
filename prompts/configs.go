package prompts

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

var additionalRelevantConfigs = []string{
	"maxMethodParams",
	"author",
}

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
	"csharp": {
		"packageName",
	},
	"unity": {
		"packageName",
	},
	"php": {
		"packageName",
		"namespace",
	},
	"ruby": {
		"packageName",
		"author",
	},
}

var ignoredKeys = []string{
	"version",
}

func PromptForTargetConfig(targetName string, target *workflow.Target, existingConfig *config.Configuration, isQuickstart bool) (*config.Configuration, error) {
	var output *config.Configuration
	if existingConfig != nil && len(existingConfig.Languages) > 0 {
		output = existingConfig
	} else {
		var err error
		output, err = config.GetDefaultConfig(true, generate.GetLanguageConfigDefaults, map[string]bool{target.Target: true})
		if err != nil {
			return nil, errors.Wrapf(err, "error generating config for target %s of type %s", targetName, target.Target)
		}
	}

	var sdkClassName string = ""
	var suggestions []string
	if !isQuickstart && output.Generation.SDKClassName != "" {
		sdkClassName = output.Generation.SDKClassName
		suggestions = append(suggestions, sdkClassName)
	} else {
		suggestions = append(suggestions, "MyCompanySDK")
	}

	_, err := charm.NewForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Name your SDK:").
					Description("your users will access SDK methods with myCompanySDK.doThing()").
					// TODO: Are there languages where the default should not be PascalCase? What about terraform?
					// TODO: If they are using the sample petstore spec during quickstart we should default to `MyPetStoreSDK`
					Placeholder("MyCompanySDK").
					Suggestions(suggestions).
					Prompt(" ").
					// TODO: Can we do better in terms of validation? ie valid identifier
					Validate(func(s string) error {
						if s == "" {
							return errors.New("SDK name must not be empty")
						}
						return nil
					}).
					Value(&sdkClassName),
			),
		)).
		ExecuteForm()
	if err != nil {
		return nil, err
	}

	var firstGroup []huh.Field
	var baseServerURL string
	if !isQuickstart && output.Generation.BaseServerURL != "" {
		baseServerURL = output.Generation.BaseServerURL
	}
	if !isQuickstart {
		firstGroup = append(firstGroup, huh.NewInput().
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

		firstGroup = append(firstGroup,
			huh.NewMultiSelect[string]().
				Title("Select your SDK Docs Languages:").
				Description("These languages will appear as options in your generated SDK Docs site.").
				Options(huh.NewOptions(generate.SupportedSDKDocsLanguages...)...).
				Value(&docsLanguages))
	}

	configGroups := []*huh.Group{
		huh.NewGroup(firstGroup...),
	}

	t, err := generate.GetTargetFromTargetString(target.Target)
	if err != nil {
		return nil, err
	}

	defaultConfigs, err := generate.GetLanguageConfigFields(t, true)
	if err != nil {
		return nil, err
	}

	languageForms, appliedKeys, err := languageSpecificForms(target.Target, output, defaultConfigs, isQuickstart, sdkClassName)
	if err != nil {
		return nil, err
	}

	configGroups = append(configGroups, languageForms...)
	form := huh.NewForm(configGroups...)
	if _, err := charm.NewForm(form,
		fmt.Sprintf("Let's configure your %s target (%s)", target.Target, targetName),
		"This will configure a config file that defines parameters for how your SDK is generated. \n"+
			"Default config values have been provided. You only need to edit values that you want to modify.").
		ExecuteForm(); err != nil {
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

	var nextState QuickstartState = Complete
	return &nextState, nil
}

func languageSpecificForms(language string, existingConfig *config.Configuration, configFields []config.SDKGenConfigField, isQuickstart bool, sdkClassName string) ([]*huh.Group, []string, error) {
	langConfig := config.LanguageConfig{}
	if existingConfig != nil {
		if conf, ok := existingConfig.Languages[language]; ok {
			langConfig = conf
		}
	}

	// Default the packageName to the SDKClassName
	// TODO: This updates the placeholder but is intended to be a default value
	// TODO: We should explain what this is by example ie:
	//    * python: pip install my-company-sdk
	//    * go: go get github.com/my-company/my-company-sdk
	//    * typescript: npm install my-company-sdk
	//if isQuickstart && (language == "go" || language == "typescript" || language == "python") {
	//	langConfig.Cfg["packageName"] = strcase.ToKebab(sdkClassName)
	//}

	groups := []*huh.Group{}
	var appliedKeys []string
	for _, field := range configFields {
		if slices.Contains(ignoredKeys, field.Name) {
			continue
		}

		if valid, defaultValue, validateRegex, validateMessage := getValuesForField(field, langConfig); valid {
			if lang, ok := quickstartScopedKeys[language]; (ok && slices.Contains(lang, field.Name)) || (!isQuickstart && slices.Contains(additionalRelevantConfigs, field.Name)) {
				appliedKeys = append(appliedKeys, field.Name)
				groups = append(groups, addPromptForField(field.Name, defaultValue, validateRegex, validateMessage, field.Description, isQuickstart))
			}
		}
	}

	return groups, appliedKeys, nil
}

func getValuesForField(field config.SDKGenConfigField, langConfig config.LanguageConfig) (bool, string, string, string) {
	defaultValue := ""
	if field.Name == "maxMethodParams" {
	}
	if field.DefaultValue != nil {
		// We only support string and boolean fields at this particular moment, more to come.
		switch val := (*field.DefaultValue).(type) {
		case string:
			defaultValue = val
		case int:
			defaultValue = strconv.Itoa(val)
		case int64:
			defaultValue = strconv.FormatInt(val, 10)
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
		case int64:
			defaultValue = strconv.FormatInt(val, 10)
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

func addPromptForField(key, defaultValue, validateRegex, validateMessage string, description *string, isQuickstart bool) *huh.Group {
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

	if defaultValue != "" {
		input = input.Value(&defaultValue)
	} else {
		input = input.Placeholder(defaultValue).Suggestions([]string{defaultValue})
	}

	return huh.NewGroup(input)
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
				case int64:
					if transform, err := strconv.Atoi(form.GetString(key)); err == nil {
						configuration.Languages[language].Cfg[key] = int64(transform)
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
