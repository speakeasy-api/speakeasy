package prompts

import (
	"fmt"
	"os"
	"strings"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

var priorityTargets = []string{
	"typescript",
	"python",
	"go",
	"java",
	"terraform",
	"csharp",
	"unity",
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

func GetTargetOptions() []huh.Option[string] {
	options := []huh.Option[string]{}

	targets := generate.GetSupportedTargets()

	// priority ordering
	for _, target := range priorityTargets {
		for _, supportedTarget := range targets {
			if supportedTarget.Target == target {
				options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", supportedTarget.Target, supportedTarget.Maturity), supportedTarget.Target))
				break
			}
		}
	}

	for _, target := range targets {
		if inPriorityTargets(target.Target) || target.Target == "docs" {
			continue
		}

		options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", target.Target, target.Maturity), target.Target))
	}

	return options
}

func GetSupportedTargets() []string {
	targets := generate.GetSupportedLanguages()
	filteredTargets := []string{}

	filteredTargets = append(filteredTargets, priorityTargets...)

	for _, language := range targets {
		if strings.HasSuffix(language, "v2") || inPriorityTargets(language) || language == "docs" {
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

func HasExistingGeneration(dir string) bool {
	if _, err := os.Stat(dir + "/gen.yaml"); err == nil {
		return true
	}

	if _, err := os.Stat(dir + "/.speakeasy/gen.yaml"); err == nil {
		return true
	}

	return false
}
