package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/gram"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

type DeployFlags struct {
	Target    string `json:"target"`
	Project   string `json:"project"`
	Directory string `json:"directory"`
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
		flag.StringFlag{
			Name:        "directory",
			Shorthand:   "d",
			Description: "MCP server directory (overrides workflow.yaml output)",
		},
	},
}

func deployExec(ctx context.Context, flags DeployFlags) error {
	l := log.From(ctx)

	if !gram.IsInstalled() {
		l.Info("Gram CLI not found.")
		if !interactivity.SimpleConfirm("Install Gram CLI now? (required for deployment)", true) {
			return fmt.Errorf("Gram CLI is required for deployment. Install from https://www.getgram.ai")
		}
		if err := gram.InstallCLI(ctx); err != nil {
			return fmt.Errorf("failed to install Gram CLI: %w", err)
		}
	}

	wf, projectDir, err := utils.GetWorkflowAndDir()
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

	outputDir := flags.Directory
	if outputDir == "" {
		if target.Output != nil {
			outputDir = *target.Output
		} else {
			outputDir = targetName
		}
	}
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(projectDir, outputDir)
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
	if result.URL != "" {
		l.Infof("URL: %s", result.URL)
	}

	return nil
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
