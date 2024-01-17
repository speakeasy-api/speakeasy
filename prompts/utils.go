package prompts

import (
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

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
