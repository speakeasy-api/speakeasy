package prompts

import (
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
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
	"php",
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
				options = append(options, targetOption(supportedTarget.Target, string(supportedTarget.Maturity)))
				break
			}
		}
	}

	for _, target := range targets {
		if inPriorityTargets(target.Target) || target.Target == "docs" {
			continue
		}

		options = append(options, targetOption(target.Target, string(target.Maturity)))
	}

	return options
}

func getTargetMaturity(target string) string {
	for _, supportedTarget := range generate.GetSupportedTargets() {
		if supportedTarget.Target == target {
			return string(supportedTarget.Maturity)
		}
	}

	return ""
}

func targetOption(target, maturity string) huh.Option[string] {
	return huh.NewOption(fmt.Sprintf("%s %s", getTargetDisplay(target), getMaturityDisplay(maturity)), target)
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
			sources = append(sources, input.Location.Reference())
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

func getTargetDisplay(target string) string {
	switch target {
	case "typescript":
		return "TypeScript"
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "java":
		return "Java"
	case "terraform":
		return "Terraform"
	case "csharp":
		return "C#"
	case "unity":
		return "Unity"
	case "php":
		return "PHP"
	case "postman":
		return "Postman"
	case "ruby":
		return "Ruby"
	case "swift":
		return "Swift"
	}

	return target
}

func getMaturityDisplay(maturity string) string {
	switch maturity {
	case "GA":
		return ""
	case "Beta":
		return styles.DimmedItalic.Render("beta")
	case "Alpha":
		return styles.DimmedItalic.Render("alpha")
	}

	return maturity
}
