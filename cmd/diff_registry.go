package cmd

import (
	"context"
	"fmt"

	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

// DiffFlags for the registry subcommand
type DiffFlags struct {
	Org          string `json:"org"`
	Workspace    string `json:"workspace"`
	Namespace    string `json:"namespace"`
	OldDigest    string `json:"old"`
	NewDigest    string `json:"new"`
	OutputDir    string `json:"output-dir"`
	Lang         string `json:"lang"`
	FormatToYAML bool   `json:"format-to-yaml"`
}

const diffRegistryLong = `# Diff Registry

Compare two OpenAPI spec revisions from the Speakeasy registry and show SDK-level changes.

This command will:
1. Download the old spec revision from the registry
2. Download the new spec revision from the registry
3. Compute and display SDK-level changes between them

Example usage:
` + "```bash" + `
speakeasy diff registry \
  --namespace my-api \
  --old sha256:abc123... \
  --new sha256:def456...

# Use a specific language for SDK diff context
speakeasy diff registry --namespace myns --old sha256:abc... --new sha256:def... --lang typescript
` + "```"

var diffRegistryCmd = &model.ExecutableCommand[DiffFlags]{
	Usage:        "registry",
	Short:        "Compare specs by registry namespace and digests",
	Long:         utils.RenderMarkdown(diffRegistryLong),
	Run:          runDiffRegistry,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "org",
			Description: "Organization slug (defaults to current)",
		},
		flag.StringFlag{
			Name:        "workspace",
			Description: "Workspace slug (defaults to current)",
		},
		flag.StringFlag{
			Name:        "namespace",
			Description: "Source namespace",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "old",
			Description: "Old revision digest (e.g., sha256:abc123...)",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "new",
			Description: "New revision digest (e.g., sha256:abc123...)",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory to download specs to",
			DefaultValue: "/tmp/speakeasy-diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
		flag.BooleanFlag{
			Name:         "format-to-yaml",
			Description:  "Pre-format specs to YAML before diffing (helps with consistent output)",
			DefaultValue: true,
		},
	},
}

func runDiffRegistry(ctx context.Context, flags DiffFlags) error {
	// Use current org/workspace if not provided
	org := flags.Org
	workspace := flags.Workspace
	if org == "" {
		org = core.GetOrgSlugFromContext(ctx)
	}
	if workspace == "" {
		workspace = core.GetWorkspaceSlugFromContext(ctx)
	}
	if org == "" || workspace == "" {
		return fmt.Errorf("org and workspace must be provided via flags or authenticated context")
	}

	return executeDiff(ctx, DiffParams{
		Org:          org,
		Workspace:    workspace,
		Namespace:    flags.Namespace,
		OldDigest:    flags.OldDigest,
		NewDigest:    flags.NewDigest,
		OutputDir:    flags.OutputDir,
		Lang:         flags.Lang,
		FormatToYAML: flags.FormatToYAML,
	})
}
