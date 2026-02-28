package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type fanoutFinalizeFlags struct {
	GithubAccessToken  string `json:"github-access-token"`
	BaseBranch         string `json:"base-branch"`
	WorkerBranches     string `json:"worker-branches"`
	TargetBranch       string `json:"target-branch"`
	ReportsDir         string `json:"reports-dir"`
	CleanupPaths       string `json:"cleanup-paths"`
	PostGenerateScript string `json:"post-generate-script"`
	CommitMessage      string `json:"commit-message"`
}

var fanoutFinalizeCmd = &model.ExecutableCommand[fanoutFinalizeFlags]{
	Usage: "fanout-finalize",
	Short: "Finalize parallel generation by aggregating worker commits into one PR branch",
	Long: `Cherry-picks commits from per-target worker branches onto a base branch, aggregates
changelog/report data, removes ephemeral changelog artifacts, squashes to one commit,
force-pushes the PR branch, and creates or updates the PR.`,
	Run: runFanoutFinalize,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for repository operations",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "base-branch",
			Description:  "Base branch the workflow is running from",
			DefaultValue: os.Getenv("INPUT_BASE_BRANCH"),
		},
		flag.StringFlag{
			Name:         "worker-branches",
			Description:  "Comma/newline-separated worker branches to collect",
			DefaultValue: os.Getenv("INPUT_WORKER_BRANCHES"),
			Required:     true,
		},
		flag.StringFlag{
			Name:         "target-branch",
			Description:  "PR target branch to force-update (if empty, resolve/create automatically)",
			DefaultValue: os.Getenv("INPUT_TARGET_BRANCH"),
		},
		flag.StringFlag{
			Name:         "reports-dir",
			Description:  "Directory containing per-target generation reports",
			DefaultValue: os.Getenv("INPUT_REPORTS_DIR"),
		},
		flag.StringFlag{
			Name:         "cleanup-paths",
			Description:  "Comma/newline-separated ephemeral paths to remove before final commit",
			DefaultValue: os.Getenv("INPUT_CLEANUP_PATHS"),
		},
		flag.StringFlag{
			Name:         "post-generate-script",
			Description:  "Optional script to run after collecting worker commits and before squashing",
			DefaultValue: os.Getenv("INPUT_POST_GENERATE_SCRIPT"),
		},
		flag.StringFlag{
			Name:         "commit-message",
			Description:  "Squashed commit message override",
			DefaultValue: os.Getenv("INPUT_COMMIT_MESSAGE"),
		},
	},
}

func runFanoutFinalize(ctx context.Context, flags fanoutFinalizeFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_BASE_BRANCH", flags.BaseBranch)
	setEnvIfNotEmpty("INPUT_WORKER_BRANCHES", flags.WorkerBranches)
	setEnvIfNotEmpty("INPUT_TARGET_BRANCH", flags.TargetBranch)
	setEnvIfNotEmpty("INPUT_REPORTS_DIR", flags.ReportsDir)
	setEnvIfNotEmpty("INPUT_CLEANUP_PATHS", flags.CleanupPaths)
	setEnvIfNotEmpty("INPUT_POST_GENERATE_SCRIPT", flags.PostGenerateScript)
	setEnvIfNotEmpty("INPUT_COMMIT_MESSAGE", flags.CommitMessage)

	return actions.FanoutFinalize(ctx, actions.FanoutFinalizeInputs{
		BaseBranch:         flags.BaseBranch,
		WorkerBranches:     flags.WorkerBranches,
		TargetBranch:       flags.TargetBranch,
		ReportsDir:         flags.ReportsDir,
		CleanupPaths:       flags.CleanupPaths,
		PostGenerateScript: flags.PostGenerateScript,
		CommitMessage:      flags.CommitMessage,
	})
}
