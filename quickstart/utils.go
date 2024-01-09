package quickstart

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

func newBranchCondition(title string) (bool, error) {
	var value bool
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title(title).
		Affirmative("Yes.").
		Negative("No.").
		Value(&value))).WithTheme(theme).Run(); err != nil {
		return false, err
	}

	return value, nil
}

func getSourcesFromWorkflow(inputWorkflow *workflow.Workflow) []string {
	var sources []string
	for key := range inputWorkflow.Sources {
		sources = append(sources, key)
	}
	return sources
}

func getSupportedTargets() []string {
	targets := generate.GetSupportedLanguages()
	filteredTargets := []string{}

	for _, language := range targets {
		if !strings.HasSuffix(language, "v2") {
			filteredTargets = append(filteredTargets, language)
		}
	}

	return filteredTargets
}
