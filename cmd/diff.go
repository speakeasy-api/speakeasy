package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	changes "github.com/speakeasy-api/openapi-generation/v2/pkg/changes"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/registry"
)

type DiffFlags struct {
	Org       string `json:"org"`
	Workspace string `json:"workspace"`
	Namespace string `json:"namespace"`
	OldDigest string `json:"old"`
	NewDigest string `json:"new"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
	NoDiff    bool   `json:"no-diff"`
}

const diffLong = `# Diff

Compare two OpenAPI spec revisions from the Speakeasy registry and show SDK-level changes.

This command will:
1. Download the old spec revision from the registry
2. Download the new spec revision from the registry
3. Compute and display SDK-level changes between them

Example usage:
` + "```bash" + `
speakeasy diff \
  --namespace my-api \
  --old sha256:abc123... \
  --new sha256:def456...

# Just download specs without showing SDK diff
speakeasy diff --org myorg --workspace myws --namespace myns --old sha256:abc... --new sha256:def... --no-diff

# Use a specific language for SDK diff context
speakeasy diff --org myorg --workspace myws --namespace myns --old sha256:abc... --new sha256:def... --lang typescript
` + "```"

var diffCmd = &model.ExecutableCommand[DiffFlags]{
	Usage:        "diff",
	Short:        "Compare two spec revisions and show SDK changes",
	Long:         utils.RenderMarkdown(diffLong),
	Run:          runDiff,
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
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
		flag.BooleanFlag{
			Name:        "no-diff",
			Description: "Just download specs, don't compute SDK diff",
		},
	},
}

func runDiff(ctx context.Context, flags DiffFlags) error {
	logger := log.From(ctx)

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

	// Clean up and prepare output directory
	oldDir := filepath.Join(flags.OutputDir, "old")
	newDir := filepath.Join(flags.OutputDir, "new")

	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("failed to create old spec directory: %w", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("failed to create new spec directory: %w", err)
	}

	// Build registry URLs
	oldLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		org, workspace, flags.Namespace, flags.OldDigest)
	newLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		org, workspace, flags.Namespace, flags.NewDigest)

	// Download old spec
	logger.Infof("Downloading old spec: %s", truncateDigest(flags.OldDigest))
	oldDoc := workflow.Document{Location: workflow.LocationString(oldLocation)}
	oldResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, oldDoc, oldDir)
	if err != nil {
		return fmt.Errorf("failed to download old spec: %w", err)
	}

	// Download new spec
	logger.Infof("Downloading new spec: %s", truncateDigest(flags.NewDigest))
	newDoc := workflow.Document{Location: workflow.LocationString(newLocation)}
	newResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, newDoc, newDir)
	if err != nil {
		return fmt.Errorf("failed to download new spec: %w", err)
	}

	logger.Infof("Specs downloaded to: %s", flags.OutputDir)

	if flags.NoDiff {
		logger.Infof("Old spec: %s", oldResult.LocalFilePath)
		logger.Infof("New spec: %s", newResult.LocalFilePath)
		return nil
	}

	// Compute SDK diff
	logger.Infof("")
	logger.Infof("Computing SDK changes (%s)...", flags.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: oldResult.LocalFilePath,
		NewSpecPath: newResult.LocalFilePath,
		OutputDir:   flags.OutputDir,
		Lang:        flags.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Output results
	logger.Infof("")
	printDiffSeparator(logger, flags.Namespace)

	if len(diff.Changes) == 0 {
		logger.Infof("No SDK-level changes detected")
	} else {
		markdown := changes.ToMarkdown(diff, changes.DetailLevelFull)
		fmt.Println(markdown)
	}

	printDiffSeparator(logger, "")

	logger.Infof("")
	logger.Infof("Old spec: %s", oldResult.LocalFilePath)
	logger.Infof("New spec: %s", newResult.LocalFilePath)

	return nil
}

func truncateDigest(digest string) string {
	// Show first 12 chars of the hash portion
	if hash, found := strings.CutPrefix(digest, "sha256:"); found {
		if len(hash) > 12 {
			return "sha256:" + hash[:12] + "..."
		}
	}
	if len(digest) > 20 {
		return digest[:20] + "..."
	}
	return digest
}

func printDiffSeparator(logger log.Logger, title string) {
	if title != "" {
		logger.Infof("SDK Changes (%s):", title)
	}
	logger.Infof("────────────────────────────────────────")
}
