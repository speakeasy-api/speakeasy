package prompts

import (
	"fmt"
	"os"
	"slices"

	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/targets"
)

var prioritySDKTargets = []string{
	"typescript",
	"python",
	"go",
	"java",
	"csharp",
	"php",
	"ruby",
}

func getSourcesFromWorkflow(inputWorkflow *workflow.Workflow) []string {
	var sources []string
	for key := range inputWorkflow.Sources {
		sources = append(sources, key)
	}
	return sources
}

func getMCPTargetOptions() []huh.Option[string] {
	options := []huh.Option[string]{}
	targets := generate.GetSupportedMCPTargets()

	for _, target := range targets {
		if target.Target == "mcp-typescript" {
			options = append(options, huh.NewOption("TypeScript Server "+getMaturityDisplay(string(target.Maturity)), "mcp-typescript"))
		}
	}

	return options
}

func getSDKTargetOptions() []huh.Option[string] {
	options := []huh.Option[string]{}
	targets := generate.GetSupportedSDKTargets()

	// priority ordering
	for _, target := range prioritySDKTargets {
		for _, supportedTarget := range targets {
			if supportedTarget.Target == target {
				options = append(options, targetOption(supportedTarget.Target, string(supportedTarget.Maturity)))
				break
			}
		}
	}

	for _, target := range targets {
		if slices.Contains(prioritySDKTargets, target.Target) {
			continue
		}

		options = append(options, targetOption(target.Target, string(target.Maturity)))
	}

	return options
}

func getTerraformTargetOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Go", "terraform"),
	}
}

func getTargetMaturity(target string) string {
	return targets.GetTargetNameMaturity(target)
}

func targetOption(target, maturity string) huh.Option[string] {
	return huh.NewOption(fmt.Sprintf("%s %s", getTargetDisplay(target), getMaturityDisplay(maturity)), target)
}

func GetSupportedTargetNames() []string {
	return targets.GetTargets()
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
