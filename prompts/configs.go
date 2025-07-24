package prompts

import (
	"context"
	"fmt"
	"slices"
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
		"modulePath",
		"sdkPackageName",
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

	if quickstart == nil || quickstart.SDKName == "" {
		initialFields = append(initialFields, createSDKNamePrompt(&sdkClassName, suggestions))
	} else {
		sdkClassName = strcase.ToCamel(quickstart.SDKName)
	}

	var baseServerURL string
	if !isQuickstart && output.Generation.BaseServerURL != "" {
		baseServerURL = output.Generation.BaseServerURL
	}
	if !isQuickstart && target.Target != "postman" {
		initialFields = append(initialFields, createBaseServerURLPrompt(&baseServerURL))
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

	targetFormGroups, targetFormFields, err := TargetSpecificForms(target.Target, output, defaultConfigs, quickstart, sdkClassName)
	if err != nil {
		return nil, err
	}

	if len(targetFormGroups) > 0 {
		form := huh.NewForm(targetFormGroups...)
		if _, err := charm.NewForm(form, charm.WithTitle(formTitle), charm.WithDescription(formSubtitle)).
			ExecuteForm(); err != nil {
			return nil, err
		}

		saveLanguageConfigValues(target.Target, output, targetFormFields)
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

func TargetSpecificForms(
	targetName string,
	existingConfig *config.Configuration,
	configFields []config.SDKGenConfigField,
	quickstart *Quickstart,
	sdkClassName string,
) ([]*huh.Group, TargetFormFields, error) {
	langConfig := config.LanguageConfig{}
	if existingConfig != nil {
		if conf, ok := existingConfig.Languages[targetName]; ok {
			langConfig = conf
		}
	}

	var groups []*huh.Group
	targetFormFields := make(TargetFormFields)

	if quickstart != nil && quickstart.SkipInteractive {
		return groups, targetFormFields, nil
	}

	isQuickstart := quickstart != nil
	targetQuickstartFieldNames, ok := quickstartScopedKeys[targetName]

	if !ok {
		targetQuickstartFieldNames = []string{}
	}

	for _, field := range configFields {
		if slices.Contains(ignoredKeys, field.Name) {
			continue
		}

		if isQuickstart && !slices.Contains(targetQuickstartFieldNames, field.Name) {
			continue
		}

		if !isQuickstart && !slices.Contains(additionalRelevantConfigs, field.Name) {
			continue
		}

		_, err := targetFormFields.Add(field, langConfig, targetName, sdkClassName, quickstart)

		if err != nil {
			return groups, targetFormFields, err
		}
	}

	// configFields ordering is non-deterministic, so its important to collect
	// the fields then deterministically add them to the form for fields that
	// intentionally reference previously answered fields.
	for _, name := range targetQuickstartFieldNames {
		if field, ok := targetFormFields[name]; ok {
			groups = append(groups, huh.NewGroup(field.HuhField(targetFormFields)))
		}
	}

	for _, name := range additionalRelevantConfigs {
		if field, ok := targetFormFields[name]; ok {
			groups = append(groups, huh.NewGroup(field.HuhField(targetFormFields)))
		}
	}

	return groups, targetFormFields, nil
}

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

func saveLanguageConfigValues(
	targetName string,
	configuration *config.Configuration,
	targetFormFields TargetFormFields,
) {
	for fieldName, targetFormField := range targetFormFields {
		switch value := targetFormField.Value.(type) {
		case *bool:
			configuration.Languages[targetName].Cfg[fieldName] = fromPointer(value)
		case *int:
			configuration.Languages[targetName].Cfg[fieldName] = fromPointer(value)
		case *int64:
			configuration.Languages[targetName].Cfg[fieldName] = fromPointer(value)
		case *string:
			configuration.Languages[targetName].Cfg[fieldName] = fromPointer(value)
		}
	}
}

// Returns the configuration field for a given name, or nil if not found.
func getSDKGenConfigField(fields []config.SDKGenConfigField, fieldName string) *config.SDKGenConfigField {
	for i := range fields {
		if fields[i].Name == fieldName {
			return &fields[i]
		}
	}

	return nil
}
