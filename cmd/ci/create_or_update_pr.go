package ci

import (
	"context"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type CreateOrUpdatePRFlags struct {
	Input  string `json:"input"`
	Branch string `json:"branch"`
}

var createOrUpdatePRCmd = &model.ExecutableCommand[CreateOrUpdatePRFlags]{
	Usage: "create-or-update-pr",
	Short: "Create or update a PR from accumulated generation reports",
	Long: `Create or update a GitHub PR from accumulated per-target generation reports.

Reads an accumulated reports JSON file (keyed by target name), merges all
version reports, generates a PR title and body, and creates or updates the PR.

The input file format is a JSON object where keys are target names and values
are per-target generation report objects containing version_report, URLs, etc.

Environment variables:
  - INPUT_GITHUB_ACCESS_TOKEN: GitHub token for API access
  - GITHUB_REPOSITORY_OWNER: Repository owner
  - GITHUB_REPOSITORY: Full repo path (owner/repo)
  - GITHUB_WORKFLOW: Workflow name (used in PR title)`,
	Run: runCreateOrUpdatePR,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "input",
			Shorthand:   "i",
			Description: "Path to accumulated reports JSON file",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "branch",
			Shorthand:   "b",
			Description: "Branch name for the PR head",
			Required:    true,
		},
	},
}

func runCreateOrUpdatePR(ctx context.Context, flags CreateOrUpdatePRFlags) error {
	return actions.CreateOrUpdatePR(ctx, flags.Input, flags.Branch)
}
