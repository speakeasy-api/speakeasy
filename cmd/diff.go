package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	changes "github.com/speakeasy-api/openapi-generation/v2/pkg/changes"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/pkg/transform"
	"github.com/speakeasy-api/speakeasy/registry"
)

const diffLong = `# Diff

Compare OpenAPI spec revisions and show **SDK-level changes** - how the generated
SDK methods, models, and types would differ between two spec versions.

This is different from ` + "`speakeasy openapi diff`" + ` which shows raw OpenAPI schema
changes (paths, operations, properties). Use this command when you want to understand
the impact on your generated SDK code.

This command supports three modes:

## Files Mode (Local)
Compare two spec files directly from disk:
` + "```bash" + `
speakeasy diff files --old old-spec.yaml --new new-spec.yaml
` + "```" + `

## From PR Mode
Look up specs from a GitHub pull request created by Speakeasy:
` + "```bash" + `
speakeasy diff from-pr https://github.com/org/repo/pull/123
` + "```" + `

## Registry Mode
Compare specs by providing registry namespace and digest values:
` + "```bash" + `
speakeasy diff registry --namespace my-api --old sha256:abc... --new sha256:def...
` + "```"

var diffCmd = &model.CommandGroup{
	Usage:          "diff",
	Short:          "Compare spec revisions and show SDK changes",
	Long:           utils.RenderMarkdown(diffLong),
	InteractiveMsg: "How would you like to look up the diff?",
	Commands:       []model.Command{diffFilesCmd, diffFromPRCmd, diffRegistryCmd},
}

// DiffParams contains the parameters needed to execute a diff from registry
type DiffParams struct {
	Org                  string
	Workspace            string
	Namespace            string
	OldDigest            string
	NewDigest            string
	OutputDir            string
	Lang                 string
	FormatToYAML         bool    // Pre-format specs to YAML before diffing (helps with consistent output)
	GenerateConfigPreRaw *string // Raw gen.yaml content from before the change (optional)
	GenerateConfigPostRaw *string // Raw gen.yaml content from after the change (optional)
}

// LocalDiffParams contains the parameters needed to execute a diff from local files
type LocalDiffParams struct {
	OldSpecPath  string
	NewSpecPath  string
	OutputDir    string
	Lang         string
	FormatToYAML bool // Pre-format specs to YAML before diffing (helps with consistent output)
}

// DiffComputeParams contains the common parameters for computing a diff
type DiffComputeParams struct {
	OldSpecPath           string
	NewSpecPath           string
	OutputDir             string
	Lang                  string
	Title                 string  // Title for the diff output (e.g., namespace or filename)
	GenerateConfigOldRaw  *string // Raw gen.yaml content for old config (optional)
	GenerateConfigNewRaw  *string // Raw gen.yaml content for new config (optional)
}

// DiffComputeResult contains the output paths from a diff computation
type DiffComputeResult struct {
	ChangesMarkdownPath string
	ChangesCompactPath  string
	ChangesHTMLPath     string
}

// computeAndOutputDiff is the shared logic for computing SDK diff and writing output files
func computeAndOutputDiff(ctx context.Context, params DiffComputeParams) (*DiffComputeResult, error) {
	logger := log.From(ctx)

	// Write generate config files if provided (used by the changes library)
	speakeasyDir := filepath.Join(params.OutputDir, ".speakeasy")
	if params.GenerateConfigOldRaw != nil || params.GenerateConfigNewRaw != nil {
		if err := os.MkdirAll(speakeasyDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create .speakeasy directory: %w", err)
		}
	}
	if params.GenerateConfigOldRaw != nil {
		genOldPath := filepath.Join(speakeasyDir, "gen.old.yaml")
		if err := os.WriteFile(genOldPath, []byte(*params.GenerateConfigOldRaw), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write gen.old.yaml: %w", err)
		}
		logger.Infof("Wrote old generate config to %s", genOldPath)
	}
	if params.GenerateConfigNewRaw != nil {
		genPath := filepath.Join(speakeasyDir, "gen.yaml")
		if err := os.WriteFile(genPath, []byte(*params.GenerateConfigNewRaw), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write gen.yaml: %w", err)
		}
		logger.Infof("Wrote new generate config to %s", genPath)
	}

	logger.Infof("")
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
		return nil, fmt.Errorf("failed to compute SDK changes: %w", err)
	}

	// Generate output in multiple formats
	markdownFull := ""
	markdownCompact := ""
	if len(diff.Changes) == 0 {
		markdownFull = "No SDK-level changes detected"
		markdownCompact = "No SDK-level changes detected"
	} else {
		markdownFull = changes.ToMarkdown(diff, changes.DetailLevelFull)
		markdownCompact = changes.ToMarkdown(diff, changes.DetailLevelCompact)
	}

	// Write changes.md (full detail)
	changesMarkdownPath := filepath.Join(params.OutputDir, "changes.md")
	if err := os.WriteFile(changesMarkdownPath, []byte(markdownFull), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write changes.md: %w", err)
	}

	// Write changes-compact.md
	changesCompactPath := filepath.Join(params.OutputDir, "changes-compact.md")
	if err := os.WriteFile(changesCompactPath, []byte(markdownCompact), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write changes-compact.md: %w", err)
	}

	// Write changes.html
	html := changes.ToHTML(diff)
	changesHTMLPath := filepath.Join(params.OutputDir, "changes.html")
	if err := os.WriteFile(changesHTMLPath, []byte(html), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write changes.html: %w", err)
	}

	// Output results to console
	logger.Infof("")
	printDiffSeparator(logger, params.Title)
	fmt.Println(markdownFull)
	printDiffSeparator(logger, "")

	return &DiffComputeResult{
		ChangesMarkdownPath: changesMarkdownPath,
		ChangesCompactPath:  changesCompactPath,
		ChangesHTMLPath:     changesHTMLPath,
	}, nil
}

// executeDiff performs the actual diff operation using registry specs
func executeDiff(ctx context.Context, params DiffParams) error {
	logger := log.From(ctx)

	// Create temp directories for bundle extraction
	oldTempDir, err := os.MkdirTemp("", "speakeasy-diff-old-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory for old spec: %w", err)
	}
	defer os.RemoveAll(oldTempDir)

	newTempDir, err := os.MkdirTemp("", "speakeasy-diff-new-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory for new spec: %w", err)
	}
	defer os.RemoveAll(newTempDir)

	// Build registry URLs
	oldLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.OldDigest)
	newLocation := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s",
		params.Org, params.Workspace, params.Namespace, params.NewDigest)

	// Download old spec
	logger.Infof("Downloading old spec: %s", truncateDigest(params.OldDigest))
	oldDoc := workflow.Document{Location: workflow.LocationString(oldLocation)}
	oldResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, oldDoc, oldTempDir)
	if err != nil {
		return fmt.Errorf("failed to download old spec: %w", err)
	}

	// Download new spec
	logger.Infof("Downloading new spec: %s", truncateDigest(params.NewDigest))
	newDoc := workflow.Document{Location: workflow.LocationString(newLocation)}
	newResult, err := registry.ResolveSpeakeasyRegistryBundle(ctx, newDoc, newTempDir)
	if err != nil {
		return fmt.Errorf("failed to download new spec: %w", err)
	}

	// Prepare output directory
	if err := os.MkdirAll(params.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Format specs to YAML and copy to output directory
	oldSpecPath := filepath.Join(params.OutputDir, "old.openapi.yaml")
	newSpecPath := filepath.Join(params.OutputDir, "new.openapi.yaml")

	logger.Infof("Formatting and copying specs...")
	if err := formatAndCopySpec(ctx, oldResult.LocalFilePath, oldSpecPath); err != nil {
		return fmt.Errorf("failed to process old spec: %w", err)
	}
	if err := formatAndCopySpec(ctx, newResult.LocalFilePath, newSpecPath); err != nil {
		return fmt.Errorf("failed to process new spec: %w", err)
	}

	// Compute and output SDK diff
	result, err := computeAndOutputDiff(ctx, DiffComputeParams{
		OldSpecPath:          oldSpecPath,
		NewSpecPath:          newSpecPath,
		OutputDir:            params.OutputDir,
		Lang:                 params.Lang,
		Title:                params.Namespace,
		GenerateConfigOldRaw: params.GenerateConfigPreRaw,
		GenerateConfigNewRaw: params.GenerateConfigPostRaw,
	})
	if err != nil {
		return err
	}

	logger.Infof("")
	logger.Infof("Registry:")
	logger.Infof("  Old: https://%s", oldLocation)
	logger.Infof("  New: https://%s", newLocation)

	logger.Infof("")
	logger.Infof("Output files:")
	logger.Infof("  %s", oldSpecPath)
	logger.Infof("  %s", newSpecPath)
	logger.Infof("  %s", result.ChangesMarkdownPath)
	logger.Infof("  %s", result.ChangesCompactPath)
	logger.Infof("  %s", result.ChangesHTMLPath)

	return nil
}

// formatAndCopySpec formats a spec to YAML and copies it to the destination path
func formatAndCopySpec(ctx context.Context, srcPath, dstPath string) error {
	formattedPath, err := formatSpecToYAML(ctx, srcPath)
	if err != nil {
		return err
	}

	// Read formatted content and write to destination
	content, err := os.ReadFile(formattedPath)
	if err != nil {
		return fmt.Errorf("failed to read formatted spec: %w", err)
	}

	if err := os.WriteFile(dstPath, content, 0o644); err != nil {
		return fmt.Errorf("failed to write spec: %w", err)
	}

	return nil
}

// executeLocalDiff performs the diff operation using local spec files directly
func executeLocalDiff(ctx context.Context, params LocalDiffParams) error {
	logger := log.From(ctx)

	// Create output directory for intermediate files
	if err := os.MkdirAll(params.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy specs to output directory (formatting to YAML if requested)
	oldSpecPath := filepath.Join(params.OutputDir, "old.openapi.yaml")
	newSpecPath := filepath.Join(params.OutputDir, "new.openapi.yaml")

	if params.FormatToYAML {
		logger.Infof("Formatting specs to YAML...")
		if err := formatAndCopySpec(ctx, params.OldSpecPath, oldSpecPath); err != nil {
			return fmt.Errorf("failed to format old spec: %w", err)
		}
		if err := formatAndCopySpec(ctx, params.NewSpecPath, newSpecPath); err != nil {
			return fmt.Errorf("failed to format new spec: %w", err)
		}
	} else {
		// Copy specs without formatting
		if err := copyFile(params.OldSpecPath, oldSpecPath); err != nil {
			return fmt.Errorf("failed to copy old spec: %w", err)
		}
		if err := copyFile(params.NewSpecPath, newSpecPath); err != nil {
			return fmt.Errorf("failed to copy new spec: %w", err)
		}
	}

	// Compute and output SDK diff
	result, err := computeAndOutputDiff(ctx, DiffComputeParams{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Title:       filepath.Base(params.NewSpecPath),
	})
	if err != nil {
		return err
	}

	logger.Infof("")
	logger.Infof("Output files:")
	logger.Infof("  %s", oldSpecPath)
	logger.Infof("  %s", newSpecPath)
	logger.Infof("  %s", result.ChangesMarkdownPath)
	logger.Infof("  %s", result.ChangesCompactPath)
	logger.Infof("  %s", result.ChangesHTMLPath)

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

// formatSpecToYAML formats an OpenAPI spec to YAML and writes it to a new file
// Returns the path to the formatted file (same directory, .yaml extension)
func formatSpecToYAML(ctx context.Context, specPath string) (string, error) {
	// Determine output path - replace extension with .yaml
	dir := filepath.Dir(specPath)
	base := filepath.Base(specPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	outputPath := filepath.Join(dir, name+".formatted.yaml")

	// Format the spec
	var buf bytes.Buffer
	if err := transform.FormatDocument(ctx, specPath, true, &buf); err != nil {
		return "", fmt.Errorf("failed to format spec: %w", err)
	}

	// Write the formatted spec
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("failed to write formatted spec: %w", err)
	}

	return outputPath, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
