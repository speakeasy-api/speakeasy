package ci

import (
	"context"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/ci/actions"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type resolveBranchFlags struct {
	GithubAccessToken string `json:"github-access-token"`
	Mode              string `json:"mode"`
	FeatureBranch     string `json:"feature-branch"`
	Debug             bool   `json:"debug"`
}

var resolveBranchCmd = &model.ExecutableCommand[resolveBranchFlags]{
	Usage: "resolve-branch",
	Short: "Resolve the target branch for SDK generation (used by CI/CD)",
	Long:  "Finds an existing open PR branch or creates a new one. Outputs branch_name for use by parallel matrix jobs via INPUT_BRANCH_NAME.",
	Run:   runResolveBranch,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "github-access-token",
			Description:  "GitHub access token for repository operations",
			DefaultValue: os.Getenv("INPUT_GITHUB_ACCESS_TOKEN"),
		},
		flag.StringFlag{
			Name:         "mode",
			Description:  "Generation mode: direct, pr, or test",
			DefaultValue: os.Getenv("INPUT_MODE"),
		},
		flag.StringFlag{
			Name:         "feature-branch",
			Description:  "Feature branch name for PR mode",
			DefaultValue: os.Getenv("INPUT_FEATURE_BRANCH"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug logging",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
	},
}

func runResolveBranch(ctx context.Context, flags resolveBranchFlags) error {
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_MODE", flags.Mode)
	setEnvIfNotEmpty("INPUT_FEATURE_BRANCH", flags.FeatureBranch)
	setEnvBool("INPUT_DEBUG", flags.Debug)

	return actions.ResolveBranch()
}
