package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/gram"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type DeployFlags struct {
	Target  string `json:"target"`
	Project string `json:"project"`
}

var deployCmd = &model.ExecutableCommand[DeployFlags]{
	Usage:        "deploy",
	Short:        "Deploy an MCP server to Gram",
	Long:         "Deploy a generated MCP server to Gram for hosting. Requires the Gram CLI to be installed.",
	Run:          deployExec,
	RequiresAuth: false,
	Experimental: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "target",
			Shorthand:   "t",
			Description: "Target name from workflow.yaml (optional if only one MCP target exists)",
		},
		flag.StringFlag{
			Name:        "project",
			Shorthand:   "p",
			Description: "Gram project name (overrides Gram default)",
		},
	},
}

func deployExec(ctx context.Context, flags DeployFlags) error {
	l := log.From(ctx)

	wf, _, err := utils.GetWorkflowAndDir()
	if err != nil {
		return fmt.Errorf("no workflow.yaml found. Run from a Speakeasy project directory")
	}

	targetName, target, err := findMCPTarget(wf, flags.Target)
	if err != nil {
		return err
	}

	if !target.DeploymentEnabled() {
		return fmt.Errorf("deployment not configured for target '%s'. Add the following to workflow.yaml:\n\ntargets:\n  %s:\n    deployment: {}", targetName, targetName)
	}

	outputDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	project := flags.Project
	if project == "" && target.Deployment != nil && target.Deployment.Project != "" {
		project = target.Deployment.Project
	}

	if project != "" {
		l.Infof("Deploying target '%s' to Gram project '%s'", targetName, project)
	} else {
		l.Infof("Deploying target '%s' to Gram (using default project)", targetName)
	}

	if err := gram.CheckAuth(ctx); err != nil {
		l.Info("Not authenticated with Gram. Starting authentication...")
		if err := gram.Auth(ctx); err != nil {
			return fmt.Errorf("failed to authenticate with Gram: %w", err)
		}
	}

	// Read package.json to show version info early
	pkg, err := gram.ReadPackageJSON(outputDir)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}
	slug := gram.DeriveSlug(pkg.Name)
	l.Infof("Version: %s@%s", slug, pkg.Version)

	if err := gram.Build(ctx, outputDir); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	result, err := gram.Push(ctx, outputDir, project)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	l.Println("")
	if result.AlreadyExists {
		l.PrintfStyled(styles.Info, "Version %s already deployed", result.Version)
	} else {
		l.PrintfStyled(styles.Success, "Deployment successful!")
	}

	mcpURL, err := setupToolset(ctx, slug, project, outputDir)
	if err != nil {
		l.Warnf("Failed to setup public MCP server: %v", err)
		if result.URL != "" {
			l.Infof("Deployment URL: %s", result.URL)
		}
	} else {
		l.PrintfStyled(styles.Success, "MCP server is live!")
		l.Infof("MCP URL: %s", mcpURL)
	}

	return nil
}

func setupToolset(ctx context.Context, slug, projectOverride, outputDir string) (string, error) {
	l := log.From(ctx)

	apiKey, err := gram.GetAPIKey()
	if err != nil {
		return "", fmt.Errorf("failed to get API key: %w", err)
	}

	projectSlug := projectOverride
	if projectSlug == "" {
		projectSlug, err = gram.GetProjectSlug()
		if err != nil {
			return "", fmt.Errorf("failed to get project slug: %w", err)
		}
	}

	orgSlug, err := gram.GetOrgSlug()
	if err != nil {
		return "", fmt.Errorf("failed to get org slug: %w", err)
	}

	zipPath := filepath.Join(outputDir, "dist", "gram.zip")
	manifest, err := gram.ReadManifestFromZip(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest: %w", err)
	}

	toolUrns := gram.BuildToolURNs(slug, manifest)
	resourceUrns := gram.BuildResourceURNs(slug, manifest)

	apiURL := gram.GetAPIURL()
	client := gram.NewToolsetsClient()

	l.Infof("Setting up MCP server '%s'...", slug)
	existingToolset, err := client.GetToolset(ctx, apiKey, projectSlug, slug)

	var toolsetSlug string
	if err != nil {
		l.Info("Creating new toolset...")
		toolset, createErr := client.CreateToolset(ctx, apiKey, projectSlug, gram.CreateToolsetParams{
			Name:        slug,
			Description: fmt.Sprintf("MCP server for %s", slug),
		})
		if createErr != nil {
			return "", fmt.Errorf("failed to create toolset: %w", createErr)
		}
		toolsetSlug = string(toolset.Slug)
		l.Infof("Created toolset: %s", toolsetSlug)
	} else {
		toolsetSlug = string(existingToolset.Slug)
		l.Infof("Using existing toolset: %s", toolsetSlug)
	}

	if len(toolUrns) > 0 || len(resourceUrns) > 0 {
		l.Infof("Adding %d tools and %d resources to toolset...", len(toolUrns), len(resourceUrns))
		_, err = client.UpdateToolsetTools(ctx, apiKey, projectSlug, toolsetSlug, toolUrns, resourceUrns)
		if err != nil {
			return "", fmt.Errorf("failed to add tools to toolset: %w", err)
		}
	}

	mcpSlug := fmt.Sprintf("%s-%s", orgSlug, slug)

	l.Info("Enabling MCP...")
	_, err = client.EnableToolset(ctx, apiKey, projectSlug, toolsetSlug, mcpSlug)
	if err != nil {
		return "", fmt.Errorf("failed to enable toolset: %w", err)
	}

	l.Info("Making MCP server public...")
	_, err = client.MakeToolsetPublic(ctx, apiKey, projectSlug, toolsetSlug)
	if err != nil {
		return "", fmt.Errorf("failed to make toolset public: %w", err)
	}

	mcpURL := fmt.Sprintf("%s/mcp/%s", apiURL, mcpSlug)
	return mcpURL, nil
}

func findMCPTarget(wf *workflow.Workflow, targetName string) (string, *workflow.Target, error) {
	if len(wf.Targets) == 0 {
		return "", nil, fmt.Errorf("no targets found in workflow.yaml")
	}

	if targetName != "" {
		target, ok := wf.Targets[targetName]
		if !ok {
			available := make([]string, 0, len(wf.Targets))
			for name := range wf.Targets {
				available = append(available, name)
			}
			return "", nil, fmt.Errorf("target '%s' not found. Available: %v", targetName, available)
		}
		if target.Target != "mcp-typescript" {
			return "", nil, fmt.Errorf("target '%s' is type '%s', not mcp-typescript", targetName, target.Target)
		}
		return targetName, &target, nil
	}

	var foundName string
	var foundTarget *workflow.Target
	mcpCount := 0

	for name, target := range wf.Targets {
		if target.Target == "mcp-typescript" {
			mcpCount++
			if foundTarget == nil {
				foundName = name
				t := target // avoid pointer to loop var
				foundTarget = &t
			}
		}
	}

	if mcpCount == 0 {
		return "", nil, fmt.Errorf("no mcp-typescript target found in workflow.yaml")
	}

	if mcpCount > 1 {
		available := make([]string, 0)
		for name, target := range wf.Targets {
			if target.Target == "mcp-typescript" {
				available = append(available, name)
			}
		}
		return "", nil, fmt.Errorf("multiple MCP targets found. Specify one with --target: %v", available)
	}

	return foundName, foundTarget, nil
}
