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

// DiffFlags for the registry subcommand
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

// FilesFlags for the files subcommand
type FilesFlags struct {
	OldSpec   string `json:"old"`
	NewSpec   string `json:"new"`
	OutputDir string `json:"output-dir"`
	Lang      string `json:"lang"`
}

const diffLong = `# Diff

Compare OpenAPI spec revisions and show SDK-level changes.

This command supports two modes:

## Files Mode (Local)
Compare two spec files directly from disk:
` + "```bash" + `
speakeasy diff files --old old-spec.yaml --new new-spec.yaml
` + "```" + `

## Registry Mode
Compare specs by providing registry namespace and digest values:
` + "```bash" + `
speakeasy diff registry --namespace my-api --old sha256:abc... --new sha256:def...
` + "```"

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

# Just download specs without showing SDK diff
speakeasy diff registry --namespace myns --old sha256:abc... --new sha256:def... --no-diff
` + "```"

const diffFilesLong = `# Diff Files

Compare two OpenAPI spec files directly from disk and show SDK-level changes.

This is the simplest mode - just provide paths to your old and new spec files.

Example usage:
` + "```bash" + `
speakeasy diff files --old ./old-openapi.yaml --new ./new-openapi.yaml

# Use a specific language for SDK diff context
speakeasy diff files --old v1.yaml --new v2.yaml --lang typescript

# Specify output directory for intermediate files
speakeasy diff files --old old.json --new new.json --output-dir ./diff-output
` + "```"

var diffCmd = &model.CommandGroup{
	Usage:          "diff",
	Short:          "Compare spec revisions and show SDK changes",
	Long:           utils.RenderMarkdown(diffLong),
	InteractiveMsg: "How would you like to look up the diff?",
	Commands:       []model.Command{diffFilesCmd, diffRegistryCmd},
}

var diffFilesCmd = &model.ExecutableCommand[FilesFlags]{
	Usage: "files",
	Short: "Compare two local spec files",
	Long:  utils.RenderMarkdown(diffFilesLong),
	Run:   runDiffFiles,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "old",
			Description: "Path to the old OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "new",
			Description: "Path to the new OpenAPI spec file",
			Required:    true,
		},
		flag.StringFlag{
			Name:         "output-dir",
			Shorthand:    "o",
			Description:  "Directory for intermediate files",
			DefaultValue: ".speakeasy/diff",
		},
		flag.StringFlag{
			Name:         "lang",
			Shorthand:    "l",
			Description:  "Target language for SDK diff context",
			DefaultValue: "go",
		},
	},
}

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

// DiffParams contains the parameters needed to execute a diff from registry
type DiffParams struct {
	Org       string
	Workspace string
	Namespace string
	OldDigest string
	NewDigest string
	OutputDir string
	Lang      string
	NoDiff    bool
}

// LocalDiffParams contains the parameters needed to execute a diff from local files
type LocalDiffParams struct {
	OldSpecPath string
	NewSpecPath string
	OutputDir   string
	Lang        string
}

func runDiffFiles(ctx context.Context, flags FilesFlags) error {
	logger := log.From(ctx)

	// Validate files exist
	if _, err := os.Stat(flags.OldSpec); os.IsNotExist(err) {
		return fmt.Errorf("old spec file not found: %s", flags.OldSpec)
	}
	if _, err := os.Stat(flags.NewSpec); os.IsNotExist(err) {
		return fmt.Errorf("new spec file not found: %s", flags.NewSpec)
	}

	// Get absolute paths
	oldSpecPath, err := filepath.Abs(flags.OldSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve old spec path: %w", err)
	}
	newSpecPath, err := filepath.Abs(flags.NewSpec)
	if err != nil {
		return fmt.Errorf("failed to resolve new spec path: %w", err)
	}

	logger.Infof("Old spec: %s", oldSpecPath)
	logger.Infof("New spec: %s", newSpecPath)
	logger.Infof("")

	return executeLocalDiff(ctx, LocalDiffParams{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
		OutputDir:   flags.OutputDir,
		Lang:        flags.Lang,
	})
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
		Org:       org,
		Workspace: workspace,
		Namespace: flags.Namespace,
		OldDigest: flags.OldDigest,
		NewDigest: flags.NewDigest,
		OutputDir: flags.OutputDir,
		Lang:      flags.Lang,
		NoDiff:    flags.NoDiff,
	})
}

// executeDiff performs the actual diff operation
func executeDiff(ctx context.Context, params DiffParams) error {
	logger := log.From(ctx)

	// Clean up and prepare output directory
	oldDir := filepath.Join(params.OutputDir, "old")
	newDir := filepath.Join(params.OutputDir, "new")

	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("failed to create old spec directory: %w", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("failed to create new spec directory: %w", err)
	}

	// Build registry URLs
	oldLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.OldDigest)
	newLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.NewDigest)

	// Download old spec
	logger.Infof("Downloading old spec: %s", truncateDigest(params.OldDigest))
	oldDoc := workflow.Document{Location: workflow.LocationString(oldLocation)}
	oldResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, oldDoc, oldDir)
	if err != nil {
		return fmt.Errorf("failed to download old spec: %w", err)
	}

	// Download new spec
	logger.Infof("Downloading new spec: %s", truncateDigest(params.NewDigest))
	newDoc := workflow.Document{Location: workflow.LocationString(newLocation)}
	newResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, newDoc, newDir)
	if err != nil {
		return fmt.Errorf("failed to download new spec: %w", err)
	}

	logger.Infof("Specs downloaded to: %s", params.OutputDir)

	if params.NoDiff {
		logger.Infof("Old spec: %s", oldResult.LocalFilePath)
		logger.Infof("New spec: %s", newResult.LocalFilePath)
		return nil
	}

	// Compute SDK diff
	logger.Infof("")
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: oldResult.LocalFilePath,
		NewSpecPath: newResult.LocalFilePath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Output results
	logger.Infof("")
	printDiffSeparator(logger, params.Namespace)

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

// executeLocalDiff performs the diff operation using local spec files directly
func executeLocalDiff(ctx context.Context, params LocalDiffParams) error {
	logger := log.From(ctx)

	// Create output directory for intermediate files
	if err := os.MkdirAll(params.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Compute SDK diff
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: params.OldSpecPath,
		NewSpecPath: params.NewSpecPath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Output results
	logger.Infof("")

	// Use the base name of the new spec as title
	title := filepath.Base(params.NewSpecPath)
	printDiffSeparator(logger, title)

	if len(diff.Changes) == 0 {
		logger.Infof("No SDK-level changes detected")
	} else {
		markdown := changes.ToMarkdown(diff, changes.DetailLevelFull)
		fmt.Println(markdown)
	}

	printDiffSeparator(logger, "")

	return nil
}
