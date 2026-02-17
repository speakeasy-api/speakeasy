package ci

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
)

type generateFlags struct {
	GithubAccessToken       string `json:"github-access-token"`
	Mode                    string `json:"mode"`
	Force                   bool   `json:"force"`
	SkipCompile             bool   `json:"skip-compile"`
	SkipTesting             bool   `json:"skip-testing"`
	SkipVersioning          bool   `json:"skip-versioning"`
	SkipRelease             bool   `json:"skip-release"`
	Target                  string `json:"target"`
	Sources                 string `json:"sources"`
	FeatureBranch           string `json:"feature-branch"`
	SetVersion              string `json:"set-version"`
	RegistryTags            string `json:"registry-tags"`
	SpeakeasyVersion        string `json:"speakeasy-version"`
	PushCodeSamplesOnly     bool   `json:"push-code-samples-only"`
	WorkingDirectory        string `json:"working-directory"`
	Debug                   bool   `json:"debug"`
	CodeSamples             string `json:"code-samples"`
	CliEnvironmentVariables string `json:"cli-environment-variables"`
	EnableSDKChangelog      string `json:"enable-sdk-changelog"`
	OpenAPIDocLocation      string `json:"openapi-doc-location"`
	SignedCommits           bool   `json:"signed-commits"`
	BranchName              string `json:"branch-name"`
}

var generateCmd = &model.ExecutableCommand[generateFlags]{
	Usage: "generate",
	Short: "Run SDK generation workflow (used by CI/CD)",
	Long:  "Runs the full SDK generation workflow. This is the CI equivalent of speakeasy run.",
	Run:   runGenerate,
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
		flag.BooleanFlag{
			Name:         "force",
			Description:  "Force generation even if no changes detected",
			DefaultValue: os.Getenv("INPUT_FORCE") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-compile",
			Description:  "Skip compilation step after generation",
			DefaultValue: os.Getenv("INPUT_SKIP_COMPILE") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-testing",
			Description:  "Skip testing step after generation",
			DefaultValue: os.Getenv("INPUT_SKIP_TESTING") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-versioning",
			Description:  "Skip version bumping",
			DefaultValue: os.Getenv("INPUT_SKIP_VERSIONING") == "true",
		},
		flag.BooleanFlag{
			Name:         "skip-release",
			Description:  "Skip release/tagging step",
			DefaultValue: os.Getenv("INPUT_SKIP_RELEASE") == "true",
		},
		flag.StringFlag{
			Name:         "target",
			Description:  "Specific target to generate (empty for all)",
			DefaultValue: os.Getenv("INPUT_TARGET"),
		},
		flag.StringFlag{
			Name:         "sources",
			Description:  "Comma-separated list of sources to generate",
			DefaultValue: os.Getenv("INPUT_SOURCES"),
		},
		flag.StringFlag{
			Name:         "feature-branch",
			Description:  "Feature branch name for PR mode",
			DefaultValue: os.Getenv("INPUT_FEATURE_BRANCH"),
		},
		flag.StringFlag{
			Name:         "set-version",
			Description:  "Explicit version to set for generated SDK",
			DefaultValue: os.Getenv("INPUT_SET_VERSION"),
		},
		flag.StringFlag{
			Name:         "registry-tags",
			Description:  "Tags for registry publishing",
			DefaultValue: os.Getenv("INPUT_REGISTRY_TAGS"),
		},
		flag.StringFlag{
			Name:         "speakeasy-version",
			Description:  "Pinned Speakeasy CLI version to use",
			DefaultValue: os.Getenv("INPUT_SPEAKEASY_VERSION"),
		},
		flag.BooleanFlag{
			Name:         "push-code-samples-only",
			Description:  "Only push code samples, skip SDK generation",
			DefaultValue: os.Getenv("INPUT_PUSH_CODE_SAMPLES_ONLY") == "true",
		},
		flag.StringFlag{
			Name:         "working-directory",
			Description:  "Working directory for generation",
			DefaultValue: os.Getenv("INPUT_WORKING_DIRECTORY"),
		},
		flag.BooleanFlag{
			Name:         "debug",
			Description:  "Enable debug logging",
			DefaultValue: os.Getenv("INPUT_DEBUG") == "true",
		},
		flag.StringFlag{
			Name:         "code-samples",
			Description:  "Code samples targets (comma-separated)",
			DefaultValue: os.Getenv("INPUT_CODE_SAMPLES"),
		},
		flag.StringFlag{
			Name:         "cli-environment-variables",
			Description:  "Additional environment variables for the CLI (dotenv format)",
			DefaultValue: os.Getenv("INPUT_CLI_ENVIRONMENT_VARIABLES"),
		},
		flag.StringFlag{
			Name:         "enable-sdk-changelog",
			Description:  "Enable SDK changelog generation",
			DefaultValue: os.Getenv("INPUT_ENABLE_SDK_CHANGELOG"),
		},
		flag.StringFlag{
			Name:         "openapi-doc-location",
			Description:  "Location of the OpenAPI document",
			DefaultValue: os.Getenv("INPUT_OPENAPI_DOC_LOCATION"),
		},
		flag.BooleanFlag{
			Name:         "signed-commits",
			Description:  "Use signed commits for git operations",
			DefaultValue: os.Getenv("INPUT_SIGNED_COMMITS") == "true",
		},
		flag.StringFlag{
			Name:         "branch-name",
			Description:  "Branch name override for generation",
			DefaultValue: os.Getenv("INPUT_BRANCH_NAME"),
		},
	},
}

func runGenerate(ctx context.Context, flags generateFlags) error {
	// Bridge flags to env vars for backward compatibility.
	// Code that reads os.Getenv("INPUT_*") will pick up values passed via CLI flags.
	setEnvIfNotEmpty("INPUT_GITHUB_ACCESS_TOKEN", flags.GithubAccessToken)
	setEnvIfNotEmpty("INPUT_MODE", flags.Mode)
	setEnvBool("INPUT_FORCE", flags.Force)
	setEnvBool("INPUT_SKIP_COMPILE", flags.SkipCompile)
	setEnvBool("INPUT_SKIP_TESTING", flags.SkipTesting)
	setEnvBool("INPUT_SKIP_VERSIONING", flags.SkipVersioning)
	setEnvBool("INPUT_SKIP_RELEASE", flags.SkipRelease)
	setEnvIfNotEmpty("INPUT_TARGET", flags.Target)
	setEnvIfNotEmpty("INPUT_SOURCES", flags.Sources)
	setEnvIfNotEmpty("INPUT_FEATURE_BRANCH", flags.FeatureBranch)
	setEnvIfNotEmpty("INPUT_SET_VERSION", flags.SetVersion)
	setEnvIfNotEmpty("INPUT_REGISTRY_TAGS", flags.RegistryTags)
	setEnvIfNotEmpty("INPUT_SPEAKEASY_VERSION", flags.SpeakeasyVersion)
	setEnvBool("INPUT_PUSH_CODE_SAMPLES_ONLY", flags.PushCodeSamplesOnly)
	setEnvIfNotEmpty("INPUT_WORKING_DIRECTORY", flags.WorkingDirectory)
	setEnvBool("INPUT_DEBUG", flags.Debug)
	setEnvIfNotEmpty("INPUT_CODE_SAMPLES", flags.CodeSamples)
	setEnvIfNotEmpty("INPUT_CLI_ENVIRONMENT_VARIABLES", flags.CliEnvironmentVariables)
	setEnvIfNotEmpty("INPUT_ENABLE_SDK_CHANGELOG", flags.EnableSDKChangelog)
	setEnvIfNotEmpty("INPUT_OPENAPI_DOC_LOCATION", flags.OpenAPIDocLocation)
	setEnvBool("INPUT_SIGNED_COMMITS", flags.SignedCommits)
	setEnvIfNotEmpty("INPUT_BRANCH_NAME", flags.BranchName)

	// TODO: delegate to actual generation logic
	// This will be implemented when the action logic is extracted
	return fmt.Errorf("ci generate: not yet implemented")
}
