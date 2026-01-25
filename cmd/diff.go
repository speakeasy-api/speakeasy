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

	// Format specs to YAML if requested
	oldSpecPath := oldResult.LocalFilePath
	newSpecPath := newResult.LocalFilePath
	if params.FormatToYAML {
		logger.Infof("Formatting specs to YAML...")
		var err error
		oldSpecPath, err = formatSpecToYAML(ctx, oldResult.LocalFilePath)
		if err != nil {
			return fmt.Errorf("failed to format old spec to YAML: %w", err)
		}
		newSpecPath, err = formatSpecToYAML(ctx, newResult.LocalFilePath)
		if err != nil {
			return fmt.Errorf("failed to format new spec to YAML: %w", err)
		}
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
	logger.Infof("Old spec: %s", oldSpecPath)
	logger.Infof("New spec: %s", newSpecPath)

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
