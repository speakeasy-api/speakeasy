package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/operations"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	core "github.com/speakeasy-api/speakeasy-core/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/speakeasy-api/speakeasy/registry"
)

type ReproFlags struct {
	ExecutionID    string `json:"execution-id"`
	Directory      string `json:"directory"`
	UseRawWorkflow bool   `json:"use-raw-workflow"`
}

const reproLong = `# Reproduce a failed generation locally

Download and reproduce a failed SDK generation locally for debugging purposes.

This command will:
1. Fetch the CLI events for the provided execution ID
2. Download the OpenAPI spec from the registry
3. Create a local reproduction environment with all necessary files
4. Automatically run 'speakeasy run' to reproduce the generation

Example usage:
` + "```bash" + `
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d --directory /tmp/debug
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d --use-raw-workflow
` + "```"

var reproCmd = &model.ExecutableCommand[ReproFlags]{
	Usage:        "repro",
	Short:        "Reproduce a failed generation locally",
	Long:         utils.RenderMarkdown(reproLong),
	Run:          runRepro,
	RequiresAuth: true,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:        "execution-id",
			Shorthand:   "e",
			Description: "Execution ID of the generation to reproduce",
			Required:    true,
		},
		flag.StringFlag{
			Name:        "directory",
			Shorthand:   "d",
			Description: "Directory to create reproduction files in (default: /tmp/{orgSlug}.{workspaceSlug})",
		},
		flag.BooleanFlag{
			Name:        "use-raw-workflow",
			Description: "Use the original workflow without downloading specs or modifying inputs",
		},
	},
}

func runRepro(ctx context.Context, flags ReproFlags) error {
	executionID := flags.ExecutionID

	// Fetch CLI events
	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SDK client: %w", err)
	}

	logger := log.From(ctx)
	logger.Infof("Fetching CLI events for execution ID: %s", executionID)

	// Search for events with this execution ID
	limit := int64(100)
	req := operations.SearchWorkspaceEventsRequest{
		ExecutionID: &executionID,
		Limit:       &limit,
	}

	res, err := s.Events.Search(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch CLI events: %w", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("API returned status code %d when searching for events", res.StatusCode)
	}

	if res.CliEventBatch == nil || len(res.CliEventBatch) == 0 {
		return fmt.Errorf("no CLI events found for execution ID: %s", executionID)
	}

	eventsForExecution := res.CliEventBatch

	// Collect interaction types for logging
	var interactionTypes []string
	for _, event := range eventsForExecution {
		interactionTypes = append(interactionTypes, string(event.InteractionType))
	}
	logger.Infof("Found %d events (%s)", len(eventsForExecution), strings.Join(interactionTypes, ", "))

	// Find the generation event and collect workflow data from any event
	var genEvent *shared.CliEvent
	var workspaceID string
	var workflowPreRaw, workflowPostRaw *string

	for _, event := range eventsForExecution {
		// Collect workflow data from any event that has it
		if event.WorkflowPreRaw != nil && workflowPreRaw == nil {
			workflowPreRaw = event.WorkflowPreRaw
		}
		if event.WorkflowPostRaw != nil && workflowPostRaw == nil {
			workflowPostRaw = event.WorkflowPostRaw
		}

		if event.InteractionType == shared.InteractionTypeTargetGenerate && event.GenerateConfigPreRaw != nil && *event.GenerateConfigPreRaw != "" {
			genEvent = &event
		}
		if event.WorkspaceID != "" {
			workspaceID = event.WorkspaceID
		}
	}

	if genEvent == nil {
		return fmt.Errorf("no generation event found for execution ID: %s (found interaction types: %s)", executionID, strings.Join(interactionTypes, ", "))
	}
	logger.Infof("Found generation event for target: %s", getValue(genEvent.GenerateTarget))

	// Fetch workspace and org info
	var orgSlug, workspaceSlug string
	if workspaceID != "" {
		wsReq := operations.GetWorkspaceRequest{
			WorkspaceID: &workspaceID,
		}
		wRes, err := s.Workspaces.GetByID(ctx, wsReq)
		if err != nil {
			logger.Warnf("Failed to fetch workspace info: %v", err)
		} else if wRes.Workspace != nil {
			if wRes.Workspace.Slug != "" {
				workspaceSlug = wRes.Workspace.Slug
			}
			// Get organization info
			if wRes.Workspace.OrganizationID != "" {
				orgReq := operations.GetOrganizationRequest{
					OrganizationID: wRes.Workspace.OrganizationID,
				}
				orgRes, err := s.Organizations.Get(ctx, orgReq)
				if err != nil {
					logger.Warnf("Failed to fetch organization info: %v", err)
				} else if orgRes.Organization != nil && orgRes.Organization.Slug != "" {
					orgSlug = orgRes.Organization.Slug
				}
			}
		}
	}

	// Determine output directory
	outputDir := flags.Directory
	if outputDir == "" {
		outputDir = filepath.Join("/tmp", fmt.Sprintf("%s.%s", orgSlug, workspaceSlug))
	}

	// Check if directory exists and clean it out
	if _, err := os.Stat(outputDir); err == nil {
		logger.Infof("Directory %s already exists, cleaning it out", outputDir)
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	logger.Infof("Creating reproduction directory: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Print generation details
	printGenerationDetails(logger, genEvent, orgSlug, workspaceSlug)

	// Create .speakeasy directory
	speakeasyDir := filepath.Join(outputDir, ".speakeasy")
	if err := os.MkdirAll(speakeasyDir, 0755); err != nil {
		return fmt.Errorf("failed to create .speakeasy directory: %w", err)
	}

	// Create logs directory and save CLI events
	logsDir := filepath.Join(speakeasyDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Save CLI events to JSON file
	eventsFile := filepath.Join(logsDir, "repro-cli-events.json")
	eventsJSON, err := json.MarshalIndent(eventsForExecution, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CLI events to JSON: %w", err)
	}
	if err := os.WriteFile(eventsFile, eventsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write CLI events to file: %w", err)
	}
	logger.Infof("Saved CLI events to %s", eventsFile)

	// Check if we should skip spec download - only skip if we don't have source info
	skipSpecDownload := genEvent.SourceNamespaceName == nil || genEvent.SourceRevisionDigest == nil

	var inputPath string
	if skipSpecDownload {
		logger.Warnf("Source namespace or revision digest missing - will use original workflow without modifications")
	} else {
		location := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s", orgSlug, workspaceSlug, *genEvent.SourceNamespaceName, *genEvent.SourceRevisionDigest)
		d := workflow.Document{
			Location: workflow.LocationString(location),
		}

		// Use the temp directory as base location for download
		tempLocation := workflow.GetTempDir()
		documentOut, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, tempLocation)
		if err != nil {
			return fmt.Errorf("failed to download spec: %w", err)
		}

		// Copy spec to input location
		inputPath = filepath.Join(speakeasyDir, "input.openapi.yaml")
		if err := utils.CopyFile(documentOut.LocalFilePath, inputPath); err != nil {
			return fmt.Errorf("failed to copy spec to input location: %w", err)
		}
	}

	// Write gen.yaml (prefer pre, fallback to post)
	genConfig := genEvent.GenerateConfigPreRaw
	if genConfig == nil || *genConfig == "" {
		return fmt.Errorf("no gen.yaml found in any event for execution ID: %s", executionID)
	}

	genPath := filepath.Join(speakeasyDir, "gen.yaml")
	if err := os.WriteFile(genPath, []byte(*genConfig), 0644); err != nil {
		return fmt.Errorf("failed to write gen.yaml: %w", err)
	}

	// Write workflow files
	// Check if we have a workflow to work with (use collected data, prefer pre, fallback to post)
	workflowRaw := workflowPreRaw
	if workflowRaw == nil {
		workflowRaw = workflowPostRaw
	}

	if workflowRaw == nil {
		return fmt.Errorf("no workflow found in any event for execution ID: %s", executionID)
	}

	// Always write the original workflow
	workflowPath := filepath.Join(speakeasyDir, "workflow.original.yaml")
	if err := os.WriteFile(workflowPath, []byte(*workflowRaw), 0644); err != nil {
		return fmt.Errorf("failed to write workflow.original.yaml: %w", err)
	}

	if skipSpecDownload || flags.UseRawWorkflow {
		// Use raw workflow without modifications
		workflowModPath := filepath.Join(speakeasyDir, "workflow.yaml")
		if err := os.WriteFile(workflowModPath, []byte(*workflowRaw), 0644); err != nil {
			return fmt.Errorf("failed to write workflow.yaml: %w", err)
		}
		logger.Infof("Using original workflow without modifications")
	} else {
		// Parse and modify workflow to use local input
		var wf workflow.Workflow
		if err := yaml.Unmarshal([]byte(*workflowRaw), &wf); err != nil {
			return fmt.Errorf("failed to parse workflow: %w", err)
		}

		// Modify all sources to point to local input
		for sourceID, source := range wf.Sources {
			source.Inputs = []workflow.Document{{Location: workflow.LocationString(".speakeasy/input.openapi.yaml")}}
			source.Overlays = nil
			source.Transformations = nil
			source.Output = nil
			source.Ruleset = nil
			source.Registry = nil
			wf.Sources[sourceID] = source
		}

		// Write modified workflow
		modifiedWorkflow, err := yaml.Marshal(&wf)
		if err != nil {
			return fmt.Errorf("failed to marshal modified workflow: %w", err)
		}

		workflowModPath := filepath.Join(speakeasyDir, "workflow.yaml")
		if err := os.WriteFile(workflowModPath, modifiedWorkflow, 0644); err != nil {
			return fmt.Errorf("failed to write workflow.yaml: %w", err)
		}
		logger.Infof("Modified workflow to use local input spec")
	}

	logger.Infof("\nRunning speakeasy run --output=console...")

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Run speakeasy run using exec
	cmd := exec.Command(execPath, "run", "--output=console")
	cmd.Dir = outputDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	_ = cmd.Run()

	logger.Infof("Reproduction files created successfully")
	logger.Infof("Location: %s", outputDir)
	logger.Infof("To reproduce the generation, run:")
	logger.Infof("  cd %s", outputDir)
	logger.Infof("  speakeasy run")

	return nil
}

func getValue(ptr *string) string {
	if ptr == nil {
		return "<nil>"
	}
	return *ptr
}

func printGenerationDetails(logger log.Logger, genEvent *shared.CliEvent, orgSlug, workspaceSlug string) {
	logger.Infof("Generation Details:")

	if !genEvent.CreatedAt.IsZero() {
		logger.Infof("  Created At: %s", genEvent.CreatedAt.Format(time.RFC3339))
	}
	if orgSlug != "" && workspaceSlug != "" {
		logger.Infof("  Workspace: https://app.speakeasy.com/org/%s/%s", orgSlug, workspaceSlug)
	}
	if genEvent.SpeakeasyAPIKeyName != "" {
		logger.Infof("  API Key Name: %s", genEvent.SpeakeasyAPIKeyName)
	}
	if genEvent.SpeakeasyVersion != "" {
		logger.Infof("  CLI Version: %s", genEvent.SpeakeasyVersion)
	}
	if genEvent.GenerateTarget != nil {
		logger.Infof("  Target: %s", *genEvent.GenerateTarget)
	}
	if genEvent.GenerateTargetName != nil {
		logger.Infof("  Target Name: %s", *genEvent.GenerateTargetName)
	}
	if genEvent.GenerateTargetVersion != nil {
		logger.Infof("  Target Version: %s", *genEvent.GenerateTargetVersion)
	}
	if genEvent.GenerateRepoURL != nil && *genEvent.GenerateRepoURL != "" {
		logger.Infof("  Repo URL: %s", *genEvent.GenerateRepoURL)
	}
	if genEvent.GhActionRunLink != nil && *genEvent.GhActionRunLink != "" {
		logger.Infof("  GitHub Action Run: %s", *genEvent.GhActionRunLink)
	}
	if genEvent.GenerateGenLockID != nil {
		logger.Infof("  Gen Lock ID: %s", *genEvent.GenerateGenLockID)
	}

	if genEvent.Success {
		logger.Infof("  Status: Success")
	} else if genEvent.Error != nil {
		logger.Infof("  Error: %s", *genEvent.Error)
	} else {
		logger.Infof("  Status: Failed")
	}

	if genEvent.SourceNamespaceName != nil {
		logger.Infof("  Source Namespace: %s", *genEvent.SourceNamespaceName)
	}

	// Check config status
	hasPreConfig := genEvent.GenerateConfigPreRaw != nil
	hasPostConfig := genEvent.GenerateConfigPostRaw != nil
	if !hasPreConfig && !hasPostConfig {
		logger.Infof("  Config: gen.yaml not found")
	} else if hasPreConfig && hasPostConfig {
		logger.Infof("  Config: gen.yaml (pre & post)")
	} else if hasPreConfig {
		logger.Infof("  Config: gen.yaml (pre only)")
	} else if hasPostConfig {
		logger.Infof("  Config: gen.yaml (post only)")
	}

	// Check workflow status
	hasPreWorkflow := genEvent.WorkflowPreRaw != nil
	hasPostWorkflow := genEvent.WorkflowPostRaw != nil
	if hasPreWorkflow && hasPostWorkflow {
		logger.Infof("  Workflow: available (pre & post)")
	} else if hasPreWorkflow {
		logger.Infof("  Workflow: available (pre only)")
	} else if hasPostWorkflow {
		logger.Infof("  Workflow: available (post only)")
	}
}
