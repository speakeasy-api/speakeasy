package prompts

import (
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

var priorityTargets = []string{
	"typescript",
	"python",
	"go",
}

func inPriorityTargets(target string) bool {
	for _, priorityTarget := range priorityTargets {
		if target == priorityTarget {
			return true
		}
	}

	return false
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
	for _, target := range priorityTargets {
		filteredTargets = append(filteredTargets, target)
	}

	for _, language := range targets {
		if strings.HasSuffix(language, "v2") || inPriorityTargets(language) {
			continue
		}
		filteredTargets = append(filteredTargets, language)
	}

	return filteredTargets
}

func getCurrentInputs(currentSource *workflow.Source) []string {
	var sources []string
	if currentSource != nil {
		for _, input := range currentSource.Inputs {
			sources = append(sources, input.Location)
		}
	}
	return sources
}
