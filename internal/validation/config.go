package validation

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

var NoConfigFound error = fmt.Errorf("no configuration found")

// GetAndValidateConfigs validates the generation config for a target and prints any errors
// Returns an error if the config is invalid, nil otherwise
func GetAndValidateConfigs(ctx context.Context) (map[string]sdkGenConfig.Config, error) {
	wf, wfDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}

	targetToConfig := make(map[string]sdkGenConfig.Config)

	err = nil
	for targetName, target := range wf.Targets {
		dir := wfDir
		if target.Output != nil && *target.Output != "" {
			dir = filepath.Join(wfDir, *target.Output)
		}

		genConfig, cfgLoadErr := sdkGenConfig.Load(dir)
		if cfgLoadErr != nil {
			return nil, cfgLoadErr
		}

		log.From(ctx).Infof("Found gen.yaml for target %s at %s\n", target.Target, genConfig.ConfigPath)

		err = ValidateConfigAndPrintErrors(ctx, target.Target, genConfig, target.IsPublished(), targetName)

		targetToConfig[target.Target] = *genConfig
	}

	if err != nil {
		return nil, fmt.Errorf("gen.yaml config is invalid. See logs above for details")
	}

	return targetToConfig, nil
}

func ValidateConfigAndPrintErrors(ctx context.Context, target string, cfg *sdkGenConfig.Config, publishingEnabled bool, targetName string) error {
	logger := log.From(ctx)
	logger.Infof("\nValidating gen.yaml for target %s...\n", target)

	logger = logger.WithFormatter(log.PrefixedFormatter)

	err := events.Telemetry(ctx, shared.InteractionTypeTargetGenerate, func(ctx context.Context, event *shared.CliEvent) error {
		event.GenerateTargetName = &targetName
		errs := ValidateConfig(target, cfg, publishingEnabled)
		if len(errs) > 0 {
			if errors.Is(errs[0], NoConfigFound) {
				return NoConfigFound
			}

			for _, err := range errs {
				logger.Error(fmt.Sprintf("%v\n", err))
			}

			return fmt.Errorf("gen.yaml config is invalid for target %s. See workflow logs for details", target)
		}

		return nil
	})

	return err
}

// ValidateConfig validates the generation config for a target and returns a list of errors
func ValidateConfig(target string, cfg *sdkGenConfig.Config, publishingEnabled bool) []error {
	if cfg == nil || cfg.Config == nil || len(cfg.Config.Languages) == 0 {
		return []error{NoConfigFound}
	} else if _, ok := cfg.Config.Languages[target]; !ok {
		return []error{fmt.Errorf("target %s not found in configuration", target)}
	}

	return ValidateTarget(target, cfg.Config.Languages[target].Cfg, publishingEnabled)
}

func ValidateTarget(target string, config map[string]any, publishingEnabled bool) []error {
	t, err := generate.GetTargetFromTargetString(target)
	if err != nil {
		return []error{err}
	}

	// TODO: newSDK???
	fields, err := generate.GetLanguageConfigFields(t, false)
	if err != nil {
		return []error{err}
	}

	var errs []error

	for _, field := range fields {
		// Special case
		if field.Name == "version" {
			continue
		}

		var msg string

		// Check required
		if field.Required {
			if _, ok := config[field.Name]; !ok {
				msg = fmt.Sprintf("%s\tfield is required", field.Name)
			}
		}

		// Check required for publishing
		if publishingEnabled && !field.Required && field.RequiredForPublishing != nil && *field.RequiredForPublishing {
			if _, ok := config[field.Name]; !ok {
				msg = fmt.Sprintf("field '%s' is required for publishing (publishing configuration was detected in your workflow file)", field.Name)
			}
		}

		// If the field is not in the config, skip the rest of the checks
		if _, ok := config[field.Name]; !ok {
			continue
		}

		// Check validation regex
		if field.ValidationRegex != nil {
			validationRegex := strings.Replace(*field.ValidationRegex, `\u002f`, `/`, -1)
			regex := regexp.MustCompile(validationRegex)

			sVal := fmt.Sprintf("%v", config[field.Name])
			if !regex.MatchString(sVal) {
				msg = fmt.Sprintf("field '%s' does not match required format", field.Name)
				if field.ValidationMessage != nil {
					msg += ": " + *field.ValidationMessage
				}
			}
		}

		// Check custom validations
		if validateFn := getValidation(t.Target, field.Name); validateFn != nil {
			if err := validateFn(config[field.Name], t.Template); err != nil {
				msg = fmt.Sprintf("field '%s' is invalid", field.Name)
				msg += ": " + err.Error()
			}
		}

		if msg != "" {
			if field.Description != nil {
				msg += ". Field description: " + *field.Description
			}

			errs = append(errs, fmt.Errorf(msg))
		}
	}

	return errs
}
