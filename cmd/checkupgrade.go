package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/checkupgrade"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

// ANSI codes
const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
	styleBold  = "\033[1m"
)

type CheckUpgradeFlags struct {
	Target string `json:"target"`
	All    bool   `json:"all"`
	Format string `json:"format"`
}

var configureGenerationCheckCmd = &model.ExecutableCommand[CheckUpgradeFlags]{
	Usage:            "check",
	Short:            "Check gen.yaml config values against newSDK defaults for targets in workflow.yaml",
	Long:             "Analyzes the gen.yaml files for SDK targets defined in workflow.yaml and compares their values against the defaults for new SDKs, identifying which settings differ from the recommended defaults.",
	Run:              checkUpgradeExec,
	UsesWorkflowFile: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "specific target to check (if not specified, checks all targets)",
		},
		flag.BooleanFlag{
			Name:        "all",
			Shorthand:   "a",
			Description: "show all config values, not just differences",
		},
		flag.EnumFlag{
			Name:          "format",
			Shorthand:     "f",
			Description:   "output format (markdown: human-readable, table: markdown table, json: JSON output)",
			AllowedValues: []string{"markdown", "table", "json"},
			DefaultValue:  "markdown",
		},
	},
}

// checkUpgradeOutput is the JSON output structure
type checkUpgradeOutput struct {
	Targets []checkUpgradeTargetOutput `json:"targets"`
}

type checkUpgradeTargetOutput struct {
	TargetID    string                       `json:"targetId"`
	Directory   string                       `json:"directory"`
	GenYamlPath string                       `json:"genYamlPath"`
	Generation  *checkUpgradeSectionOutput   `json:"generation,omitempty"`
	Languages   []checkUpgradeSectionOutput  `json:"languages,omitempty"`
}

type checkUpgradeSectionOutput struct {
	Name        string                    `json:"name"`
	Differences []checkUpgradeDiffOutput  `json:"differences"`
	Matches     []checkUpgradeDiffOutput  `json:"matches,omitempty"`
}

type checkUpgradeDiffOutput struct {
	Key          string `json:"key"`
	Description  string `json:"description,omitempty"`
	CurrentValue any    `json:"currentValue"`
	NewSDKValue  any    `json:"newSDKValue"`
}

func checkUpgradeExec(ctx context.Context, flags CheckUpgradeFlags) error {
	logger := log.From(ctx)

	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return fmt.Errorf("failed to load workflow file: %w", err)
	}

	if err := wf.Validate(generate.GetSupportedTargetNames()); err != nil {
		return fmt.Errorf("invalid workflow file: %w", err)
	}

	// Get targets to check
	var targetsToCheck []string
	if flags.Target != "" {
		if _, ok := wf.Targets[flags.Target]; !ok {
			return fmt.Errorf("target %q not found in workflow.yaml", flags.Target)
		}
		targetsToCheck = []string{flags.Target}
	} else {
		for targetID := range wf.Targets {
			targetsToCheck = append(targetsToCheck, targetID)
		}
		sort.Strings(targetsToCheck)
	}

	if len(targetsToCheck) == 0 {
		if flags.Format == "json" {
			fmt.Println("{\"targets\":[]}")
		} else {
			logger.Println("No targets found in workflow.yaml")
		}
		return nil
	}

	// Collect results for all targets
	var output checkUpgradeOutput

	for _, targetID := range targetsToCheck {
		target := wf.Targets[targetID]

		// Determine the output directory for this target
		outDir := projectDir
		if target.Output != nil && *target.Output != "" && *target.Output != "." {
			outDir = filepath.Join(projectDir, *target.Output)
		}

		// Check if directory exists
		if _, err := os.Stat(outDir); os.IsNotExist(err) {
			if flags.Format != "json" {
				logger.Printf("Skipping target %q: output directory %s does not exist\n", targetID, outDir)
			}
			continue
		}

		result, err := checkupgrade.Check(outDir, checkupgrade.Options{
			IncludeMatches: flags.All,
		})
		if err != nil {
			if flags.Format != "json" {
				logger.Printf("Error checking target %q: %v\n", targetID, err)
			}
			continue
		}

		targetOutput := checkUpgradeTargetOutput{
			TargetID:    targetID,
			Directory:   outDir,
			GenYamlPath: result.GenYamlPath,
		}

		// Process generation section
		if result.Generation != nil {
			targetOutput.Generation = convertSectionToOutput(result.Generation)
		}

		// Process language sections
		langs := make([]string, 0, len(result.Languages))
		for lang := range result.Languages {
			langs = append(langs, lang)
		}
		sort.Strings(langs)

		for _, lang := range langs {
			section := result.Languages[lang]
			targetOutput.Languages = append(targetOutput.Languages, *convertSectionToOutput(section))
		}

		output.Targets = append(output.Targets, targetOutput)
	}

	// Output based on format
	switch flags.Format {
	case "json":
		return outputJSON(output)
	case "table":
		return outputTable(logger, output, flags.All)
	default:
		return outputMarkdown(logger, output, flags.All)
	}
}

func convertSectionToOutput(section *checkupgrade.SectionResult) *checkUpgradeSectionOutput {
	out := &checkUpgradeSectionOutput{
		Name:        section.Name,
		Differences: make([]checkUpgradeDiffOutput, 0, len(section.Differences)),
		Matches:     make([]checkUpgradeDiffOutput, 0, len(section.Matches)),
	}

	for _, diff := range section.Differences {
		out.Differences = append(out.Differences, checkUpgradeDiffOutput{
			Key:          diff.Key,
			Description:  diff.Description,
			CurrentValue: diff.CurrentValue,
			NewSDKValue:  diff.NewSDKValue,
		})
	}

	for _, match := range section.Matches {
		out.Matches = append(out.Matches, checkUpgradeDiffOutput{
			Key:          match.Key,
			Description:  match.Description,
			CurrentValue: match.CurrentValue,
			NewSDKValue:  match.NewSDKValue,
		})
	}

	return out
}

func outputJSON(output checkUpgradeOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputTable(_ log.Logger, output checkUpgradeOutput, showAll bool) error {
	for i, target := range output.Targets {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("## Target: %s\n\n", target.TargetID)

		// Generation section
		if target.Generation != nil && len(target.Generation.Differences) > 0 {
			fmt.Println("### Generation")
			fmt.Println()
			fmt.Println("| Key | Current | New SDK | Description |")
			fmt.Println("|-----|---------|---------|-------------|")
			for _, diff := range target.Generation.Differences {
				fmt.Printf("| `%s` | `%v` | `%v` | %s |\n",
					diff.Key,
					checkupgrade.FormatValue(diff.CurrentValue),
					checkupgrade.FormatValue(diff.NewSDKValue),
					diff.Description)
			}
			fmt.Println()
		}

		if showAll && target.Generation != nil && len(target.Generation.Matches) > 0 {
			fmt.Println("#### Generation (Matching)")
			fmt.Println()
			fmt.Println("| Key | Value | Description |")
			fmt.Println("|-----|-------|-------------|")
			for _, match := range target.Generation.Matches {
				val := match.CurrentValue
				if val == nil {
					val = match.NewSDKValue
				}
				fmt.Printf("| `%s` | `%v` | %s |\n", match.Key, checkupgrade.FormatValue(val), match.Description)
			}
			fmt.Println()
		}

		// Language sections
		for _, lang := range target.Languages {
			if len(lang.Differences) > 0 {
				fmt.Printf("### %s\n\n", lang.Name)
				fmt.Println("| Key | Current | New SDK | Description |")
				fmt.Println("|-----|---------|---------|-------------|")
				for _, diff := range lang.Differences {
					fmt.Printf("| `%s` | `%v` | `%v` | %s |\n",
						diff.Key,
						checkupgrade.FormatValue(diff.CurrentValue),
						checkupgrade.FormatValue(diff.NewSDKValue),
						diff.Description)
				}
				fmt.Println()
			}

			if showAll && len(lang.Matches) > 0 {
				fmt.Printf("#### %s (Matching)\n\n", lang.Name)
				fmt.Println("| Key | Value | Description |")
				fmt.Println("|-----|-------|-------------|")
				for _, match := range lang.Matches {
					val := match.CurrentValue
					if val == nil {
						val = match.NewSDKValue
					}
					fmt.Printf("| `%s` | `%v` | %s |\n", match.Key, checkupgrade.FormatValue(val), match.Description)
				}
				fmt.Println()
			}
		}
	}

	return nil
}

func outputMarkdown(logger log.Logger, output checkUpgradeOutput, showAll bool) error {
	for i, target := range output.Targets {
		if i > 0 {
			logger.Println("")
		}
		logger.Printf("=== Target: %s ===\n", target.TargetID)
		logger.Printf("Checking directory: %s\n\n", target.Directory)
		logger.Printf("Found gen.yaml at: %s\n\n", target.GenYamlPath)

		// Print generation section
		if target.Generation != nil {
			logger.Println("--- Generation Section ---")
			printCheckUpgradeSection(logger, target.Generation, showAll)
			logger.Println("")
		}

		// Print language sections
		for _, lang := range target.Languages {
			logger.Printf("--- %s Section ---\n", lang.Name)
			printCheckUpgradeSection(logger, &lang, showAll)
			logger.Println("")
		}
	}

	return nil
}

func printCheckUpgradeSection(logger log.Logger, section *checkUpgradeSectionOutput, showAll bool) {
	if len(section.Differences) > 0 {
		logger.Println("Differences from newSDK defaults:")
		for i, diff := range section.Differences {
			if i > 0 {
				logger.Println("") // Add newline between config keys
			}
			if diff.Description != "" {
				logger.Printf("  %s%s%s: %s", styleBold, diff.Key, colorReset, diff.Description)
			} else {
				logger.Printf("  %s%s%s:\n", styleBold, diff.Key, colorReset)
			}
			logger.Printf("    %scurrent:  %v%s\n    %snewSDK:   %v%s\n", colorRed, checkupgrade.FormatValue(diff.CurrentValue), colorReset, colorGreen, checkupgrade.FormatValue(diff.NewSDKValue), colorReset)
		}
	} else {
		logger.Printf("%sAll values match newSDK defaults%s\n", colorGreen, colorReset)
	}

	if showAll && len(section.Matches) > 0 {
		logger.Println("\nMatching values:")
		for _, match := range section.Matches {
			if match.CurrentValue != nil {
				logger.Printf("  %s: %v (matches default)\n", match.Key, checkupgrade.FormatValue(match.CurrentValue))
			} else {
				logger.Printf("  %s: (not set, using default: %v)\n", match.Key, checkupgrade.FormatValue(match.NewSDKValue))
			}
		}
	}
}
