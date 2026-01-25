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
	Org          string
	Workspace    string
	Namespace    string
	OldDigest    string
	NewDigest    string
	OutputDir    string
	Lang         string
	NoDiff       bool
	FormatToYAML bool // Pre-format specs to YAML before diffing (helps with consistent output)
}

// LocalDiffParams contains the parameters needed to execute a diff from local files
type LocalDiffParams struct {
	OldSpecPath  string
	NewSpecPath  string
	OutputDir    string
	Lang         string
	FormatToYAML bool // Pre-format specs to YAML before diffing (helps with consistent output)
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

	if params.NoDiff {
		logger.Infof("Old spec: %s", oldSpecPath)
		logger.Infof("New spec: %s", newSpecPath)
		return nil
	}

	// Compute SDK diff
	logger.Infof("")
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
		OutputDir:   params.OutputDir,
		Lang:        params.Lang,
		Verbose:     false,
		Logger:      logger,
	})

	diff, err := changes.Changes(ctx, oldConfig, newConfig)
	if err != nil {
		return fmt.Errorf("failed to compute SDK changes: %w", err)
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
		return fmt.Errorf("failed to write changes.md: %w", err)
	}

	// Write changes-compact.md
	changesCompactPath := filepath.Join(params.OutputDir, "changes-compact.md")
	if err := os.WriteFile(changesCompactPath, []byte(markdownCompact), 0o644); err != nil {
		return fmt.Errorf("failed to write changes-compact.md: %w", err)
	}

	// Write changes.html
	html := changes.ToHTML(diff)
	changesHTMLPath := filepath.Join(params.OutputDir, "changes.html")
	if err := os.WriteFile(changesHTMLPath, []byte(html), 0o644); err != nil {
		return fmt.Errorf("failed to write changes.html: %w", err)
	}

	// Output results to console
	logger.Infof("")
	printDiffSeparator(logger, params.Namespace)
	fmt.Println(markdownFull)
	printDiffSeparator(logger, "")

	logger.Infof("")
	logger.Infof("Registry:")
	logger.Infof("  Old: https://%s", oldLocation)
	logger.Infof("  New: https://%s", newLocation)

	logger.Infof("")
	logger.Infof("Output files:")
	logger.Infof("  %s", oldSpecPath)
	logger.Infof("  %s", newSpecPath)
	logger.Infof("  %s", changesMarkdownPath)
	logger.Infof("  %s", changesCompactPath)
	logger.Infof("  %s", changesHTMLPath)

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

	// Format specs to YAML if requested
	oldSpecPath := params.OldSpecPath
	newSpecPath := params.NewSpecPath
	if params.FormatToYAML {
		logger.Infof("Formatting specs to YAML...")
		var err error
		oldSpecPath, err = formatSpecToYAML(ctx, params.OldSpecPath)
		if err != nil {
			return fmt.Errorf("failed to format old spec to YAML: %w", err)
		}
		newSpecPath, err = formatSpecToYAML(ctx, params.NewSpecPath)
		if err != nil {
			return fmt.Errorf("failed to format new spec to YAML: %w", err)
		}
	}

	// Compute SDK diff
	logger.Infof("Computing SDK changes (%s)...", params.Lang)

	oldConfig, newConfig := changes.CreateConfigsFromSpecPaths(changes.SpecComparison{
		OldSpecPath: oldSpecPath,
		NewSpecPath: newSpecPath,
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
	title := filepath.Base(newSpecPath)
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
