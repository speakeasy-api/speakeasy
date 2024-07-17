package prompts

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
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
	"terraform": {
		"packageName",
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

func PromptForTargetConfig(targetName string, wf *workflow.Workflow, target *workflow.Target, existingConfig *config.Configuration, quickstart *Quickstart) (*config.Configuration, error) {
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

	isQuickstart := quickstart != nil

	sdkClassName := ""
	var suggestions []string
	if !isQuickstart && output.Generation.SDKClassName != "" {
		sdkClassName = output.Generation.SDKClassName
		suggestions = append(suggestions, sdkClassName)
	} else {
		suggestions = append(suggestions, "MyCompanySDK")
	}

	initialFields := []huh.Field{}

	if quickstart == nil || quickstart.SDKName == "" {
		initialFields = append(initialFields,
			huh.NewInput().
				Title("Name your SDK").
				Description("This should be PascalCase. Your users will access SDK methods with myCompanySDK.doThing()\n").
				Placeholder("MyCompanySDK").
				Suggestions(suggestions).
				Prompt("").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("SDK name must not be empty")
					}
					return nil
				}).
				Value(&sdkClassName),
		)
	} else {
		sdkClassName = strcase.ToCamel(quickstart.SDKName)
	}

	var baseServerURL string
	if !isQuickstart && output.Generation.BaseServerURL != "" {
		baseServerURL = output.Generation.BaseServerURL
	}
	if !isQuickstart && target.Target != "postman" {
		initialFields = append(initialFields, huh.NewInput().
			Title("Provide a base server URL for your SDK to use:").
			Placeholder("You must do this if a server URL is not defined in your OpenAPI spec").
			Inline(true).
			Prompt(" ").
			Value(&baseServerURL))
	}

	formTitle := fmt.Sprintf("Let's configure your %s target (%s)", target.Target, targetName)
	formSubtitle := "This will configure a config file that defines parameters for how your SDK is generated. \n" +
		"Default config values have been provided. You only need to edit values that you want to modify."

	if len(initialFields) > 0 {
		form := huh.NewForm(huh.NewGroup(initialFields...))
		if _, err := charm.NewForm(form, formTitle, formSubtitle).ExecuteForm(); err != nil {
			return nil, err
		}
	}

	t, err := generate.GetTargetFromTargetString(target.Target)
	if err != nil {
		return nil, err
	}

	defaultConfigs, err := generate.GetLanguageConfigFields(t, true)
	if err != nil {
		return nil, err
	}

	languageGroups, fields, err := languageSpecificForms(target.Target, output, defaultConfigs, isQuickstart, sdkClassName)
	if err != nil {
		return nil, err
	}

	if len(languageGroups) > 0 {
		form := huh.NewForm(languageGroups...)
		if _, err := charm.NewForm(form, formTitle, formSubtitle).
			ExecuteForm(); err != nil {
			return nil, err
		}

		saveLanguageConfigValues(target.Target, form, output, fields, defaultConfigs)
	}

	output.Generation.SDKClassName = sdkClassName
	output.Generation.BaseServerURL = baseServerURL

	// default dev containers on for new SDKs
	if isQuickstart {
		setDevContainerDefaults(output, wf, target)
	}

	return output, nil
}

func setDevContainerDefaults(output *config.Configuration, wf *workflow.Workflow, target *workflow.Target) {
	if target.Target == "go" || target.Target == "typescript" || target.Target == "python" {
		if source, ok := wf.Sources[target.Source]; ok {
			schemaPath := ""
			if source.Output != nil {
				schemaPath = *source.Output
			} else {
				schemaPath = source.Inputs[0].Location
			}
			output.Generation.DevContainers = &config.DevContainers{
				Enabled:    true,
				SchemaPath: schemaPath,
			}
		}
	}
}

func configBaseForm(ctx context.Context, quickstart *Quickstart) (*QuickstartState, error) {
	for key, target := range quickstart.WorkflowFile.Targets {
		output, err := PromptForTargetConfig(key, quickstart.WorkflowFile, &target, nil, quickstart)
		if err != nil {
			return nil, err
		}

		quickstart.LanguageConfigs[key] = output
	}

	var nextState QuickstartState = Complete
	return &nextState, nil
}

type LangField struct {
	key          string
	defaultValue string
}

func languageSpecificForms(
	language string,
	existingConfig *config.Configuration,
	configFields []config.SDKGenConfigField,
	isQuickstart bool,
	sdkClassName string,
) ([]*huh.Group, []LangField, error) {
	langConfig := config.LanguageConfig{}
	if existingConfig != nil {
		if conf, ok := existingConfig.Languages[language]; ok {
			langConfig = conf
		}
	}

	var groups []*huh.Group

	var fields []LangField
	for _, field := range configFields {
		if slices.Contains(ignoredKeys, field.Name) {
			continue
		}

		valid, defaultValue, validateRegex, validateMessage, description := getValuesForField(field, langConfig, language, sdkClassName, isQuickstart)

		if valid {
			if lang, ok := quickstartScopedKeys[language]; (ok && slices.Contains(lang, field.Name)) || (!isQuickstart && slices.Contains(additionalRelevantConfigs, field.Name)) {
				fields = append(fields, LangField{
					key:          field.Name,
					defaultValue: defaultValue,
				})
				groups = append(groups, addPromptForField(field.Name, defaultValue, validateRegex, validateMessage, &description))
			}
		}
	}

	return groups, fields, nil
}

func getValuesForField(field config.SDKGenConfigField, langConfig config.LanguageConfig, language string, sdkClassName string, isQuickstart bool) (bool, string, string, string, string) {
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
			return false, "", "", "", ""
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

	description := ""
	if field.Description != nil {
		description = *field.Description
	}
	if field.Name == "packageName" && isQuickstart && sdkClassName != "" {
		switch language {
		case "go":
			defaultValue = "github.com/my-company/" + strcase.ToKebab(sdkClassName)
			description = description + "\nTo install your SDK users will execute:\ngo get " + defaultValue
		case "typescript":
			defaultValue = strcase.ToKebab(sdkClassName)
			description = description + "\nTo install your SDK users will execute:\nnpm install " + defaultValue
		case "python":
			defaultValue = strcase.ToKebab(sdkClassName)
			description = description + "\nTo install your SDK users will execute:\npip install " + defaultValue
		case "terraform":
			defaultValue = strcase.ToKebab(sdkClassName)
		}
	}

	return true, defaultValue, validationRegex, validationMessage, description
}

func addPromptForField(key, defaultValue, validateRegex, validateMessage string, description *string) *huh.Group {
	input := charm.NewInlineInput().
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
	}

	return huh.NewGroup(input)
}

func saveLanguageConfigValues(
	language string,
	form *huh.Form,
	configuration *config.Configuration,
	fields []LangField,
	configFields []config.SDKGenConfigField,
) {
	for _, formField := range fields {
		key := formField.key
		var field *config.SDKGenConfigField

		for _, f := range configFields {
			if f.Name == key {
				field = &f
				break
			}
		}
		if field != nil {
			var val any

			if field.DefaultValue != nil {
				// Use the default value if the actual value is unset
				if form.GetString(key) == "" {
					val = formField.defaultValue
				} else {
					// We need to map values back to their native type since the form only can produce a string
					switch (*field.DefaultValue).(type) {
					case int:
						val, _ = strconv.Atoi(form.GetString(key))
					case int64:
						val, _ = strconv.Atoi(form.GetString(key))
					case bool:
						val, _ = strconv.ParseBool(form.GetString(key))
					case string:
						val = form.GetString(key)
					}
				}
			} else {
				val = form.GetString(key)
			}

			configuration.Languages[language].Cfg[key] = val
		}
	}
}
