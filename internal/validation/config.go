package validation

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	sdkGenConfig "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/maps"
	"path/filepath"
	"regexp"
	"strings"
)

// GetAndValidateConfigs validates the generation config for a target and prints any errors
// Returns an error if the config is invalid, nil otherwise
func GetAndValidateConfigs(ctx context.Context) (map[string]sdkGenConfig.Config, error) {
	wf, wfDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return nil, err
	}

	targetToConfig := make(map[string]sdkGenConfig.Config)

	err = nil
	for _, target := range wf.Targets {
		dir := wfDir
		if target.Output != nil && *target.Output != "" {
			dir = filepath.Join(wfDir, *target.Output)
		}

		genConfig, cfgLoadErr := sdkGenConfig.Load(dir)
		if cfgLoadErr != nil {
			return nil, cfgLoadErr
		}

		log.From(ctx).Infof("Found gen.yaml for target %s at %s\n", target.Target, genConfig.ConfigPath)

		err = ValidateConfigAndPrintErrors(ctx, target.Target, genConfig, target.IsPublished())

		targetToConfig[target.Target] = *genConfig
	}

	if err != nil {
		return nil, fmt.Errorf("gen.yaml config is invalid. See logs above for details")
	}

	return targetToConfig, nil
}

func ValidateConfigAndPrintErrors(ctx context.Context, target string, cfg *sdkGenConfig.Config, publishingEnabled bool) error {
	logger := log.From(ctx)
	logger.Infof("\nValidating gen.yaml for target %s...\n", target)

	logger = logger.WithFormatter(log.PrefixedFormatter)

	err := events.Telemetry(ctx, shared.InteractionTypeTargetGenerate, func(ctx context.Context, event *shared.CliEvent) error {
		errs := ValidateConfig(target, cfg, publishingEnabled)
		if len(errs) > 0 {
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
		return []error{fmt.Errorf("no configuration found")}
	} else if _, ok := cfg.Config.Languages[target]; !ok {
		println("FOUND: ", strings.Join(maps.Keys(cfg.Config.Languages), ", "))
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

		// Check validation regex
		if field.ValidationRegex != nil {
			validationRegex := strings.Replace(*field.ValidationRegex, `\u002f`, `/`, -1)
			regex := regexp.MustCompile(validationRegex)

			if _, ok := config[field.Name]; !ok {
				continue
			}

			val, ok := config[field.Name]
			if !ok {
				msg = fmt.Sprintf("field '%s' is expected to be a string", field.Name)
			}

			sVal := fmt.Sprintf("%v", val)
			if !regex.MatchString(sVal) {
				msg = fmt.Sprintf("field '%s' does not match required format", field.Name)
				if field.ValidationMessage != nil {
					msg += ": " + *field.ValidationMessage
				}
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
