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
	DryRun bool   `json:"dry-run"`
}

var createOrUpdatePRCmd = &model.ExecutableCommand[CreateOrUpdatePRFlags]{
	Usage: "create-or-update-pr",
	Short: "Create or update a PR from accumulated generation reports",
	Long: `Create or update a GitHub PR from per-target generation reports.

Reads a directory of per-target report JSON files, merges all version reports,
generates a PR title and body, and creates or updates the PR.

Each file in the directory should be a JSON object with target, version_report,
speakeasy_version, etc. Files are keyed by filename (target name + .json).

Use --dry-run to print the generated title and body without creating a PR.

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
			Description: "Path to directory containing per-target report JSON files",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "branch",
			Shorthand:   "b",
			Description: "Branch name for the PR head",
		},
		flag.BooleanFlag{
			Name:        "dry-run",
			Description: "Print generated PR title and body without creating a PR",
		},
	},
}

func runCreateOrUpdatePR(ctx context.Context, flags CreateOrUpdatePRFlags) error {
	return actions.CreateOrUpdatePR(ctx, flags.Input, flags.Branch, flags.DryRun)
}
