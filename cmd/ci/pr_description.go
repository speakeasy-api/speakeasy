package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/prdescription"
)

type PRDescriptionFlags struct {
	Input string `json:"input"`
}

var prDescriptionCmd = &model.ExecutableCommand[PRDescriptionFlags]{
	Usage: "pr-description",
	Short: "Generate PR title and body for SDK update PRs",
	Long: `Generate a PR title and body from version report data.

Input is provided as JSON via --input flag (file path or "-" for stdin).
Output is JSON with "title" and "body" fields.

The input JSON structure supports these fields (all optional):
  - linting_report_url: URL to linting report
  - changes_report_url: URL to OpenAPI changes report
  - workflow_name: Name of the workflow
  - source_branch: Source branch name
  - feature_branch: Feature branch name (if applicable)
  - target: SDK target language
  - specified_target: Target specified via INPUT_TARGET
  - source_generation: Boolean for source-only generation
  - docs_generation: Boolean for docs generation
  - speakeasy_version: CLI version for footer
  - manual_bump: Boolean for manual version bump
  - version_report: MergedVersionReport data

Unknown fields are ignored for forward compatibility.`,
	Run: runPRDescription,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "input",
			Shorthand:   "i",
			Description: "Path to JSON input file, or \"-\" for stdin",
			Required:    true,
		},
	},
}

func runPRDescription(ctx context.Context, flags PRDescriptionFlags) error {
	// Read input JSON
	var inputData []byte
	var err error

	if flags.Input == "-" {
		inputData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		inputData, err = os.ReadFile(flags.Input)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}
	}

	// Parse input - unknown fields are ignored for forward compatibility
	var input prdescription.Input
	if err := json.Unmarshal(inputData, &input); err != nil {
		return fmt.Errorf("failed to parse input JSON: %w", err)
	}

	// Generate PR description
	output, err := prdescription.Generate(input)
	if err != nil {
		return fmt.Errorf("failed to generate PR description: %w", err)
	}

	// Output as JSON
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(outputJSON))
	return nil
}
