package prompts

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/speakeasy-api/huh"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

// Represents form field data for target-specific configurations.
type TargetFormField struct {
	// Function to generate a description for the field based on its value.
	// This allows for dynamic descriptions that can change based on user input.
	DescriptionFunc func(any) string

	// Configuration field name.
	Name string

	// Function to generate suggestions for the field based on other form
	// fields that were previously configured.
	SuggestionsFunc func(TargetFormFields) []string

	// Form field title. If empty, the title is set to "Choose a {{.Name}}".
	Title string

	// Value is the current value of the form field and must be a pointer type.
	// If the generation configuration has a default value for this field, that
	// value will be used as the initial value.
	Value any

	// Function to generate value for the field based on other form fields that
	// were previously configured. Must return a pointer type.
	ValueFunc func(TargetFormFields) any

	// Regular expression to validate the field value.
	ValidationRegex *regexp.Regexp

	// Message to display if the field value does not match ValidationRegex.
	ValidationMessage string
}

// Creates a new target form field based on the given generator configuration
// field.
func NewTargetFormField(
	targetFormFields TargetFormFields,
	genConfigField config.SDKGenConfigField,
	targetConfig config.LanguageConfig,
	targetName string,
	sdkClassName string,
	quickstart *Quickstart,
) (*TargetFormField, error) {
	description := ""
	isQuickstart := quickstart != nil
	targetFormField := &TargetFormField{
		Name:  genConfigField.Name,
		Value: genConfigField.DefaultValue,
	}

	if genConfigField.Description != nil {
		description = *genConfigField.Description
	}

	if value, ok := targetConfig.Cfg[targetFormField.Name]; ok {
		targetFormField.SetValue(value)
	}

	targetFormField.SetValidationMessage(genConfigField.ValidationMessage)

	if err := targetFormField.SetValidationRegex(genConfigField.ValidationRegex); err != nil {
		return targetFormField, fmt.Errorf("error setting validation regex for field %s: %w", targetFormField.Name, err)
	}

	if isQuickstart {
		switch targetFormField.Name {
		case "modulePath":
			switch targetName {
			case "go":
				description = "Root module path. To install your SDK, users will execute " + styles.Emphasized.Render("go get %s") + "\nFor additional information: https://go.dev/ref/mod#module-path"
				targetFormField.SetValue("github.com/my-company/company-go-sdk")
			}
		case "packageName":
			packageName := sdkClassName

			if quickstart.IsUsingTemplate && quickstart.Defaults.TemplateData != nil {
				packageName = quickstart.Defaults.TemplateData.PackageName
			}

			switch targetName {
			case "python":
				description = description + "\nTo install your SDK, users will execute " + styles.Emphasized.Render("pip install %s")
				targetFormField.SetValue(strcase.ToKebab(packageName))
			case "terraform":
				targetFormField.SetValue(strcase.ToKebab(packageName))
			case "typescript":
				description = description + "\nTo install your SDK, users will execute " + styles.Emphasized.Render("npm install %s")
				targetFormField.SetValue(strcase.ToKebab(packageName))
			case "mcp-typescript":
				description = "We recommend a descriptive name for your MCP Server. \nThis will be appear as the server name to MCP Clients like " + styles.Emphasized.Render("Cursor, Claude Code.")
				targetFormField.SetValue(strcase.ToKebab(packageName))
			}
		case "sdkPackageName":
			sdkPackageName := sdkClassName

			if quickstart.IsUsingTemplate && quickstart.Defaults.TemplateData != nil {
				sdkPackageName = quickstart.Defaults.TemplateData.PackageName
			}

			switch targetName {
			case "go":
				description = "Root module package name. To instantiate your SDK, users will call " + styles.Emphasized.Render("%s.New()") + "\nFor additional information: https://go.dev/ref/spec#Packages"
				if sdkPackageName != "" {
					targetFormField.SetValue(sdkPackageName)
				} else {
					targetFormField.SuggestionsFunc = func(answeredFields TargetFormFields) []string {
						modulePath := answeredFields.GetField("modulePath").GetValueString()
						sdkPackageName := goModulePathToPackageName(modulePath)

						return []string{sdkPackageName}
					}
					targetFormField.ValueFunc = func(answeredFields TargetFormFields) any {
						modulePath := answeredFields.GetField("modulePath").GetValueString()
						sdkPackageName := goModulePathToPackageName(modulePath)

						return toPointer(sdkPackageName)
					}
				}
			}
		case "cloudflareEnabled":
			description = "This will enable Cloudflare Workers deployment for your MCP Server"
			targetFormField.Title = "Do you plan on deploying to Cloudflare?"
		}
	}

	targetFormField.SetDescriptionFunc(description)

	// As last resort ensure the value is a string pointer.
	if targetFormField.Value == nil {
		targetFormField.Value = new(string)
	}

	return targetFormField, nil
}

// Returns the dereferenced string value. If the value is nil or not a string
// pointer, this returns an empty string.
func (f TargetFormField) GetValueString() string {
	value, ok := f.Value.(*string)

	if !ok || value == nil {
		return ""
	}

	return *value
}

// Returns a huh.Field representation of the target form field.
func (f *TargetFormField) HuhField(targetFormFields TargetFormFields) huh.Field {
	// TODO: This only sets the Value field at form initialization. Migrate to
	// using Accessor interface to dynamically retrieve the value at runtime.
	if f.ValueFunc != nil {
		f.Value = f.ValueFunc(targetFormFields)
	}

	switch value := f.Value.(type) {
	case *string:
		input := charm.NewInlineInput(value).Key(f.Name)

		if f.Title != "" {
			input = input.Title(f.Title)
		} else {
			input = input.Title("Choose a " + f.Name)
		}

		if f.DescriptionFunc != nil {
			fn := func() string {
				return f.DescriptionFunc(*value)
			}
			input = input.DescriptionFunc(fn, value).Inline(false).Prompt("")
		}

		if f.SuggestionsFunc != nil {
			fn := func() []string {
				return f.SuggestionsFunc(targetFormFields)
			}
			input = input.SuggestionsFunc(fn, value)
		}

		if f.ValidationRegex != nil {
			input = input.Validate(func(s string) error {
				if !f.ValidationRegex.MatchString(strings.TrimSpace(s)) {
					return errors.New(f.ValidationMessage)
				}

				return nil
			})
		}

		return input
	case *bool:
		confirm := huh.NewConfirm().Key(f.Name)

		if f.Title != "" {
			confirm = confirm.Title(f.Title)
		}

		if f.DescriptionFunc != nil {
			confirm = confirm.Description(f.DescriptionFunc(*value))
		}

		return confirm.Value(value)
	case interface{}, *interface{}:
		// unwrap f.Value
		unwrap := func(x interface{}) interface{} {
			rv := reflect.ValueOf(x)
			for rv.IsValid() && (rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr) {
				if rv.IsNil() {
					return nil
				}
				rv = rv.Elem()
			}
			return rv.Interface()
		}

		value = unwrap(f.Value)
		switch value.(type) {
		case int64:

			var intValue string

			input := charm.NewInlineInput(&intValue).Key(f.Name)
			f.Value = &intValue

			if f.Title != "" {
				input = input.Title(f.Title)
			} else {
				input = input.Title("Choose a " + f.Name)
			}

			if f.DescriptionFunc != nil {
				fn := func() string {
					return f.DescriptionFunc(intValue)
				}
				input = input.DescriptionFunc(fn, &intValue).Inline(false).Prompt("")
			}

			if f.SuggestionsFunc != nil {
				fn := func() []string {
					return f.SuggestionsFunc(targetFormFields)
				}
				input = input.SuggestionsFunc(fn, &intValue)
			}

			if f.ValidationRegex != nil {
				input = input.Validate(func(s string) error {
					if !f.ValidationRegex.MatchString(strings.TrimSpace(s)) {
						return errors.New(f.ValidationMessage)
					}

					return nil
				})
			}

			// Add validation to ensure the input is a valid integer
			input = input.Validate(func(s string) error {
				if s == "" {
					return nil // Allow empty values
				}
				_, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					return errors.New("must be a valid integer")
				}
				return nil
			})
			return input
		}
		return nil
	default:
		return nil
	}
}

// Sets the description function for the target form field. If the description
// contains a "%s" placeholder, it will format the description with the value
// passed to the function. Otherwise, it will return the description as is.
func (f *TargetFormField) SetDescriptionFunc(description string) {
	if f == nil {
		return
	}

	if strings.Contains(description, "%s") {
		f.DescriptionFunc = func(v any) string {
			switch v := v.(type) {
			case *string:
				if *v == "" {
					return fmt.Sprintf(description, "UNSET")
				}
			}

			return fmt.Sprintf(description, v)
		}

		return
	}

	f.DescriptionFunc = func(_ any) string {
		return description
	}
}

func (f *TargetFormField) SetValue(value any) {
	if f == nil || value == nil {
		return
	}

	switch v := value.(type) {
	case string:
		f.Value = toPointer(v)
	case *string:
		f.Value = v
	case bool:
		f.Value = toPointer(v)
	case *bool:
		f.Value = v
	}
}

func (f *TargetFormField) SetValidationMessage(message *string) {
	if f == nil || message == nil {
		return
	}

	f.ValidationMessage = *message
}

func (f *TargetFormField) SetValidationRegex(regex *string) error {
	if f == nil {
		return errors.New("cannot set validation regex on nil TargetFormField")
	}

	if regex == nil {
		return nil
	}

	validationRegexStr := strings.Replace(*regex, `\u002f`, `/`, -1)
	validationRegex, err := regexp.Compile(validationRegexStr)
	if err != nil {
		return fmt.Errorf("error compiling validation regex %s: %w", validationRegexStr, err)
	}

	f.ValidationRegex = validationRegex

	return nil
}
