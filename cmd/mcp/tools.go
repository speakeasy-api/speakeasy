package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func getTools() []tool {
	return []tool{
		{
			Name:        "speakeasy_generate",
			Description: "Generate an SDK from an OpenAPI specification. Supports TypeScript, Python, Go, Java, C#, PHP, Ruby, Swift, Terraform, and more.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path to the OpenAPI specification file (YAML or JSON), or a URL"
					},
					"target": {
						"type": "string",
						"description": "Target language for SDK generation",
						"enum": ["typescript", "python", "go", "java", "csharp", "php", "ruby", "swift", "kotlin", "terraform", "mcp-typescript"]
					},
					"out": {
						"type": "string",
						"description": "Output directory for the generated SDK"
					},
					"packageName": {
						"type": "string",
						"description": "Name for the generated package (optional)"
					}
				},
				"required": ["schema", "target", "out"]
			}`),
		},
		{
			Name:        "speakeasy_lint",
			Description: "Validate and lint an OpenAPI specification. Returns diagnostics including errors, warnings, and hints for improvement.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path to the OpenAPI specification file to lint"
					}
				},
				"required": ["schema"]
			}`),
		},
		{
			Name:        "speakeasy_suggest",
			Description: "Get AI-powered suggestions to improve your OpenAPI specification. Can suggest better operation IDs and error type definitions.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path to the OpenAPI specification file"
					},
					"type": {
						"type": "string",
						"description": "Type of suggestions to generate",
						"enum": ["operation-ids", "error-types"]
					},
					"output": {
						"type": "string",
						"description": "Output path for the suggested overlay file (optional)"
					}
				},
				"required": ["schema", "type"]
			}`),
		},
		{
			Name:        "speakeasy_run",
			Description: "Execute a Speakeasy workflow defined in .speakeasy/workflow.yaml. This runs the full SDK generation pipeline including any configured overlays and transformations.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"target": {
						"type": "string",
						"description": "Specific target to run from the workflow (optional, runs all if not specified)"
					},
					"source": {
						"type": "string",
						"description": "Specific source to use from the workflow (optional)"
					}
				}
			}`),
		},
		{
			Name:        "speakeasy_status",
			Description: "Check the status of the current Speakeasy workspace. Shows configured sources, targets, and their states.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "speakeasy_quickstart",
			Description: "Initialize a new Speakeasy project with a workflow configuration. Creates the .speakeasy directory structure and workflow.yaml file.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path or URL to the OpenAPI specification"
					},
					"target": {
						"type": "string",
						"description": "Target language for SDK generation",
						"enum": ["typescript", "python", "go", "java", "csharp", "php", "ruby", "swift", "kotlin", "terraform"]
					},
					"out": {
						"type": "string",
						"description": "Output directory for the SDK"
					}
				},
				"required": ["schema", "target"]
			}`),
		},
		{
			Name:        "speakeasy_overlay_create",
			Description: "Create an OpenAPI overlay template. Overlays allow you to modify an OpenAPI spec without changing the original file.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path to the base OpenAPI specification"
					},
					"output": {
						"type": "string",
						"description": "Output path for the overlay file"
					}
				},
				"required": ["schema"]
			}`),
		},
		{
			Name:        "speakeasy_overlay_apply",
			Description: "Apply an OpenAPI overlay to a specification.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schema": {
						"type": "string",
						"description": "Path to the base OpenAPI specification"
					},
					"overlay": {
						"type": "string",
						"description": "Path to the overlay file to apply"
					},
					"output": {
						"type": "string",
						"description": "Output path for the result"
					}
				},
				"required": ["schema", "overlay"]
			}`),
		},
		{
			Name:        "speakeasy_merge",
			Description: "Merge multiple OpenAPI specifications into a single unified spec.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"schemas": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Paths to the OpenAPI specifications to merge"
					},
					"output": {
						"type": "string",
						"description": "Output path for the merged specification"
					}
				},
				"required": ["schemas", "output"]
			}`),
		},
	}
}

func executeTool(ctx context.Context, name string, args map[string]any) (string, bool) {
	switch name {
	case "speakeasy_generate":
		return executeGenerate(ctx, args)
	case "speakeasy_lint":
		return executeLint(ctx, args)
	case "speakeasy_suggest":
		return executeSuggest(ctx, args)
	case "speakeasy_run":
		return executeRun(ctx, args)
	case "speakeasy_status":
		return executeStatus(ctx, args)
	case "speakeasy_quickstart":
		return executeQuickstart(ctx, args)
	case "speakeasy_overlay_create":
		return executeOverlayCreate(ctx, args)
	case "speakeasy_overlay_apply":
		return executeOverlayApply(ctx, args)
	case "speakeasy_merge":
		return executeMerge(ctx, args)
	default:
		return fmt.Sprintf("Unknown tool: %s", name), true
	}
}

func runSpeakeasyCommand(ctx context.Context, args ...string) (string, bool) {
	// Find the speakeasy executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("Failed to find speakeasy executable: %s", err), true
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		return fmt.Sprintf("Command failed: %s\n%s", err, output), true
	}

	if output == "" {
		output = "Command completed successfully."
	}

	return output, false
}

func getString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringSlice(args map[string]any, key string) []string {
	if v, ok := args[key]; ok {
		if arr, ok := v.([]any); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func executeGenerate(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	target := getString(args, "target")
	out := getString(args, "out")
	packageName := getString(args, "packageName")

	if schema == "" || target == "" || out == "" {
		return "Missing required parameters: schema, target, and out are required", true
	}

	cmdArgs := []string{"generate", "sdk", "-s", schema, "-t", target, "-o", out}
	if packageName != "" {
		cmdArgs = append(cmdArgs, "-p", packageName)
	}

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

func executeLint(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	if schema == "" {
		return "Missing required parameter: schema", true
	}

	return runSpeakeasyCommand(ctx, "lint", "openapi", "-s", schema, "--non-interactive")
}

func executeSuggest(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	suggestType := getString(args, "type")
	output := getString(args, "output")

	if schema == "" || suggestType == "" {
		return "Missing required parameters: schema and type are required", true
	}

	cmdArgs := []string{"suggest", suggestType, "-s", schema}
	if output != "" {
		cmdArgs = append(cmdArgs, "-o", output)
	}

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

func executeRun(ctx context.Context, args map[string]any) (string, bool) {
	target := getString(args, "target")
	source := getString(args, "source")

	cmdArgs := []string{"run"}
	if target != "" {
		cmdArgs = append(cmdArgs, "-t", target)
	}
	if source != "" {
		cmdArgs = append(cmdArgs, "-s", source)
	}

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

func executeStatus(ctx context.Context, args map[string]any) (string, bool) {
	return runSpeakeasyCommand(ctx, "status")
}

func executeQuickstart(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	target := getString(args, "target")
	out := getString(args, "out")

	if schema == "" || target == "" {
		return "Missing required parameters: schema and target are required", true
	}

	// Create the workflow directory
	workflowDir := ".speakeasy"
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		return fmt.Sprintf("Failed to create .speakeasy directory: %s", err), true
	}

	// Determine output directory
	outputDir := out
	if outputDir == "" {
		outputDir = filepath.Join("sdk", target)
	}

	// Create workflow.yaml
	workflowContent := fmt.Sprintf(`workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
  my-api:
    inputs:
      - location: %s
targets:
  %s-sdk:
    target: %s
    source: my-api
    output: %s
`, schema, target, target, outputDir)

	workflowPath := filepath.Join(workflowDir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644); err != nil {
		return fmt.Sprintf("Failed to write workflow.yaml: %s", err), true
	}

	return fmt.Sprintf(`Speakeasy project initialized successfully!

Created %s with:
- Source: %s
- Target: %s
- Output: %s

Next steps:
1. Run 'speakeasy run' to generate your SDK
2. Customize the workflow.yaml as needed
3. Add more targets for additional languages`, workflowPath, schema, target, outputDir), false
}

func executeOverlayCreate(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	output := getString(args, "output")

	if schema == "" {
		return "Missing required parameter: schema", true
	}

	cmdArgs := []string{"overlay", "create", "-s", schema}
	if output != "" {
		cmdArgs = append(cmdArgs, "-o", output)
	}

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

func executeOverlayApply(ctx context.Context, args map[string]any) (string, bool) {
	schema := getString(args, "schema")
	overlay := getString(args, "overlay")
	output := getString(args, "output")

	if schema == "" || overlay == "" {
		return "Missing required parameters: schema and overlay are required", true
	}

	cmdArgs := []string{"overlay", "apply", "-s", schema, "-o", overlay}
	if output != "" {
		cmdArgs = append(cmdArgs, "--out", output)
	}

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

func executeMerge(ctx context.Context, args map[string]any) (string, bool) {
	schemas := getStringSlice(args, "schemas")
	output := getString(args, "output")

	if len(schemas) == 0 || output == "" {
		return "Missing required parameters: schemas (array) and output are required", true
	}

	cmdArgs := []string{"merge", "-o", output}
	cmdArgs = append(cmdArgs, schemas...)

	return runSpeakeasyCommand(ctx, cmdArgs...)
}

// Helper for non-exported use
func init() {
	// Validate all tool schemas are valid JSON
	for _, t := range getTools() {
		var schema map[string]any
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			panic(fmt.Sprintf("Invalid schema for tool %s: %s", t.Name, err))
		}
	}
}
