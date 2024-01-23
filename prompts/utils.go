package prompts

import (
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

var priorityTargets = map[string]any{
	"typescript": nil,
	"python":     nil,
	"go":         nil,
}

func getSourcesFromWorkflow(inputWorkflow *workflow.Workflow) []string {
	var sources []string
	for key := range inputWorkflow.Sources {
		sources = append(sources, key)
	}
	return sources
}

func GetSupportedTargets() []string {
	targets := generate.GetSupportedLanguages()
	filteredTargets := []string{}

	// priority ordering
	for key := range priorityTargets {
		filteredTargets = append(filteredTargets, key)
	}

	for _, language := range targets {
		if _, ok := priorityTargets[language]; !ok && !strings.HasSuffix(language, "v2") {
			filteredTargets = append(filteredTargets, language)
		}
	}

	return filteredTargets
}
