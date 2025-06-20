package prompts

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/speakeasy-api/speakeasy/internal/charm/styles"

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
	"mcp-typescript": {
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

	// Check if SDK name is provided via hidden flag
	if quickstart != nil && quickstart.SDKName != "" {
		if quickstart.SDKName == DefaultOptionFlag {
			sdkClassName = "MyCompanySDK"
		} else {
			sdkClassName = quickstart.SDKName
		}
	} else if quickstart == nil || quickstart.SDKName == "" {
		initialFields = append(initialFields, createSDKNamePrompt(&sdkClassName, suggestions))
	} else {
		sdkClassName = strcase.ToCamel(quickstart.SDKName)
	}

	var baseServerURL string
	// Check if base server URL is provided via hidden flag
	if quickstart != nil && quickstart.BaseServerURL != "" {
		if quickstart.BaseServerURL == DefaultOptionFlag {
			baseServerURL = ""
		} else {
			baseServerURL = quickstart.BaseServerURL
		}
	} else {
		if !isQuickstart && output.Generation.BaseServerURL != "" {
			baseServerURL = output.Generation.BaseServerURL
		}
		if !isQuickstart && target.Target != "postman" {
			initialFields = append(initialFields, createBaseServerURLPrompt(&baseServerURL))
		}
	}

	formTitle := fmt.Sprintf("Let's configure your %s target (%s)", target.Target, targetName)
	formSubtitle := "This will configure a config file that defines parameters for how your SDK is generated. \n" +
		"Default config values have been provided. You only need to edit values that you want to modify."

	if len(initialFields) > 0 {
		form := huh.NewForm(huh.NewGroup(initialFields...))
		if _, err := charm.NewForm(form, charm.WithTitle(formTitle), charm.WithDescription(formSubtitle)).ExecuteForm(); err != nil {
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

	languageGroups, fields, err := languageSpecificForms(target.Target, output, defaultConfigs, quickstart, sdkClassName)
	if err != nil {
		return nil, err
	}

	if len(languageGroups) > 0 {
		form := huh.NewForm(languageGroups...)
		if _, err := charm.NewForm(form, charm.WithTitle(formTitle), charm.WithDescription(formSubtitle)).
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
		setEnvVarPrefixDefaults(output, target, sdkClassName)
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
				schemaPath = source.Inputs[0].Location.Resolve()
			}
			output.Generation.DevContainers = &config.DevContainers{
				Enabled:    true,
				SchemaPath: schemaPath,
			}
		}
	}
}

func setEnvVarPrefixDefaults(output *config.Configuration, target *workflow.Target, sdkClassName string) {
	if target.Target == "go" || target.Target == "typescript" || target.Target == "python" || target.Target == "mcp-typescript" {
		if cfg, ok := output.Languages[target.Target]; ok && cfg.Cfg != nil {
			cfg.Cfg["envVarPrefix"] = strings.ToUpper(sdkClassName)
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
	quickstart *Quickstart,
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

		valid, defaultValue, validateRegex, validateMessage, descriptionFn := getValuesForField(field, langConfig, language, sdkClassName, quickstart)
		isQuickstart := quickstart != nil

		if valid {
			if lang, ok := quickstartScopedKeys[language]; (ok && slices.Contains(lang, field.Name)) || (!isQuickstart && slices.Contains(additionalRelevantConfigs, field.Name)) {
				fields = append(fields, LangField{
					key:          field.Name,
					defaultValue: defaultValue,
				})
				
				// Add prompt only if not provided via hidden flag
				if !shouldSkipFieldPrompt(quickstart, field.Name) {
					groups = append(groups, addPromptForField(field.Name, defaultValue, validateRegex, validateMessage, descriptionFn))
				}
			}
		}
	}

	return groups, fields, nil
}

func getValuesForField(
	field config.SDKGenConfigField,
	langConfig config.LanguageConfig,
	language string,
	sdkClassName string,
	quickstart *Quickstart,
) (
	valid bool,
	defaultValue string,
	validationRegex string,
	validationMessage string,
	descriptionFn func(v string) string,
) {
	isQuickstart := quickstart != nil
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
			return false, "", "", "", nil
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

	if field.ValidationRegex != nil {
		validationRegex = *field.ValidationRegex
		validationRegex = strings.Replace(validationRegex, `\u002f`, `/`, -1)
	}

	if field.ValidationRegex != nil {
		validationMessage = *field.ValidationMessage
	}

	description := ""
	if field.Description != nil {
		description = *field.Description
	}
	if field.Name == "packageName" && isQuickstart {
		// Check if package name is provided via hidden flag
		if quickstart.PackageName != "" {
			if quickstart.PackageName == DefaultOptionFlag {
				// Use default logic
				packageName := sdkClassName
				if quickstart.IsUsingTemplate && quickstart.Defaults.TemplateData != nil {
					packageName = quickstart.Defaults.TemplateData.PackageName
				}
				
				switch language {
				case "go":
					defaultValue = "github.com/my-company/" + strcase.ToKebab(packageName)
				case "typescript", "python":
					defaultValue = strcase.ToKebab(packageName)
				case "terraform":
					defaultValue = strcase.ToKebab(packageName)
				default:
					defaultValue = strcase.ToKebab(packageName)
				}
			} else {
				defaultValue = quickstart.PackageName
			}
		} else {
			// By default we base the package name on the SDK class name
			packageName := sdkClassName

			if quickstart.IsUsingTemplate && quickstart.Defaults.TemplateData != nil {
				packageName = quickstart.Defaults.TemplateData.PackageName
			}

			switch language {
			case "go":
				defaultValue = "github.com/my-company/" + strcase.ToKebab(packageName)
			case "typescript":
				defaultValue = strcase.ToKebab(packageName)
			case "python":
				defaultValue = strcase.ToKebab(packageName)
			case "terraform":
				defaultValue = strcase.ToKebab(packageName)
			}
		}
		
		switch language {
		case "go":
			description = description + "\nTo install your SDK, users will execute " + styles.Emphasized.Render("go get %s")
		case "typescript":
			description = description + "\nTo install your SDK, users will execute " + styles.Emphasized.Render("npm install %s")
		case "python":
			description = description + "\nTo install your SDK, users will execute " + styles.Emphasized.Render("pip install %s")
		}
	}

	// Handle other language-specific hidden flags
	if isQuickstart {
		switch field.Name {
		case "groupID":
			if language == "java" && quickstart.GroupID != "" {
				if quickstart.GroupID == DefaultOptionFlag {
					defaultValue = "com.example"
				} else {
					defaultValue = quickstart.GroupID
				}
			}
		case "artifactID":
			if language == "java" && quickstart.ArtifactID != "" {
				if quickstart.ArtifactID == DefaultOptionFlag {
					defaultValue = strcase.ToKebab(sdkClassName)
				} else {
					defaultValue = quickstart.ArtifactID
				}
			}
		case "namespace":
			if language == "php" && quickstart.Namespace != "" {
				if quickstart.Namespace == DefaultOptionFlag {
					defaultValue = strcase.ToCamel(sdkClassName)
				} else {
					defaultValue = quickstart.Namespace
				}
			}
		case "author":
			if language == "ruby" && quickstart.Author != "" {
				if quickstart.Author == DefaultOptionFlag {
					defaultValue = "SDK Team"
				} else {
					defaultValue = quickstart.Author
				}
			}
		}
	}

	descriptionFn = func(v string) string {
		if strings.Contains(description, "%s") {
			return fmt.Sprintf(description, v)
		} else {
			return description
		}
	}

	valid = true
	return
}

func addPromptForField(key, defaultValue, validateRegex, validateMessage string, descriptionFn func(v string) string) *huh.Group {
	value := defaultValue

	input := charm.NewInlineInput(&value).
		Key(key).
		Title(fmt.Sprintf("Choose a %s", key)).
		Validate(func(s string) error {
			if validateRegex != "" {
				s = strings.TrimSpace(s)
				s = strings.Trim(s, "\n")
				s = strings.Trim(s, "\t")
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

	if descriptionFn != nil {
		fn := func() string {
			return descriptionFn(value)
		}
		input = input.DescriptionFunc(fn, &value).Inline(false).Prompt("")
	}

	return huh.NewGroup(input)
}

// Helper functions to reduce nesting and handle DEFAULT flag short-circuiting

func createSDKNamePrompt(sdkClassName *string, suggestions []string) huh.Field {
	return huh.NewInput().
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
		Value(sdkClassName)
}

func createBaseServerURLPrompt(baseServerURL *string) huh.Field {
	return huh.NewInput().
		Title("Provide a base server URL for your SDK to use:").
		Placeholder("You must do this if a server URL is not defined in your OpenAPI spec").
		Inline(true).
		Prompt(" ").
		Value(baseServerURL)
}

func shouldSkipFieldPrompt(quickstart *Quickstart, fieldName string) bool {
	if quickstart == nil {
		return false
	}
	
	switch fieldName {
	case "packageName":
		return quickstart.PackageName != ""
	case "groupID":
		return quickstart.GroupID != ""
	case "artifactID":
		return quickstart.ArtifactID != ""
	case "namespace":
		return quickstart.Namespace != ""
	case "author":
		return quickstart.Author != ""
	}
	return false
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
			formValue := form.GetString(key)
			formValue = strings.TrimSpace(formValue)
			formValue = strings.Trim(formValue, "\n")
			formValue = strings.Trim(formValue, "\t")
			if field.DefaultValue != nil {
				// Use the default value if the actual value is unset
				if formValue == "" {
					val = formField.defaultValue
				} else {
					// We need to map values back to their native type since the form only can produce a string
					switch (*field.DefaultValue).(type) {
					case int:
						val, _ = strconv.Atoi(formValue)
					case int64:
						val, _ = strconv.Atoi(formValue)
					case bool:
						val, _ = strconv.ParseBool(formValue)
					case string:
						val = formValue
					}
				}
			} else {
				val = formValue
			}

			configuration.Languages[language].Cfg[key] = val
		}
	}
}
