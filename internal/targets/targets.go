package targets

import (
	"slices"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	config "github.com/speakeasy-api/sdk-gen-config"
)

const DocsTarget = "docs"
const docsDefaultVersion = "0.0.1"
const docsMaturity = ""

type Target struct {
	Target   string
	Template string
	Maturity string
}

func GetTargets() []string {
	targetNames := slices.Clone(generate.GetSupportedTargetNames())
	if !slices.Contains(targetNames, DocsTarget) {
		targetNames = append(targetNames, DocsTarget)
	}
	slices.Sort(targetNames)
	return targetNames
}

func GetTargetFromTargetString(target string) (Target, error) {
	if target == DocsTarget {
		return Target{
			Target:   DocsTarget,
			Template: DocsTarget,
			Maturity: docsMaturity,
		}, nil
	}

	t, err := generate.GetTargetFromTargetString(target)
	if err != nil {
		return Target{}, err
	}

	return Target{
		Target:   t.Target,
		Template: t.Template,
		Maturity: string(t.Maturity),
	}, nil
}

func GetLanguageConfigFields(target Target, newSDK bool) ([]config.SDKGenConfigField, error) {
	if target.Target == DocsTarget {
		return []config.SDKGenConfigField{}, nil
	}

	t, err := generate.GetTargetFromTargetString(target.Target)
	if err != nil {
		return nil, err
	}

	return generate.GetLanguageConfigFields(t, newSDK)
}

func GetLanguageConfigDefaults(target string, newSDK bool) (*config.LanguageConfig, error) {
	if target == DocsTarget {
		return &config.LanguageConfig{
			Version: docsDefaultVersion,
			Cfg:     map[string]any{},
		}, nil
	}

	return generate.GetLanguageConfigDefaults(target, newSDK)
}

func GetTargetNameMaturity(targetName string) string {
	if targetName == DocsTarget {
		return docsMaturity
	}

	return generate.GetTargetNameMaturity(targetName)
}
