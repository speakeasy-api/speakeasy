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

const (
	inputSpecFilename = "snapshotted.openapi.yaml"
	reproLong         = `# Reproduce a failed generation locally

Download and reproduce a failed SDK generation locally for debugging purposes.

This command will:
1. Fetch the CLI events for the provided execution ID
2. Download the merged/overlayed OpenAPI spec that was actually used for generation (default)
3. Create a local reproduction environment with all necessary files
4. Automatically run 'speakeasy run' to reproduce the generation

By default, this command downloads the final merged spec that was used for generation.
Use --use-raw-workflow if you need to debug overlay or workflow source issues.

Example usage:
` + "```bash" + `
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d --directory /tmp/debug
speakeasy repro --execution-id c303282d-f2e6-46ca-a04a-35d3d873712d --use-raw-workflow  # Debug workflow/overlay issues
` + "```"
)

type slugs struct {
	org       string
	workspace string
}

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
			Description: "Use the original workflow to debug overlay/source issues (default: use merged spec)",
		},
	},
}

func runRepro(ctx context.Context, flags ReproFlags) error {
	executionID := flags.ExecutionID
	logger := log.From(ctx)

	eventsForExecution, err := fetchCLIEvents(ctx, executionID)
	if err != nil {
		return err
	}

	interactionTypes := collectInteractionTypes(eventsForExecution)
	logger.Infof("Found %d events (%s)", len(eventsForExecution), strings.Join(interactionTypes, ", "))

	genEvent := findGenEvent(eventsForExecution)
	if genEvent == nil {
		return fmt.Errorf("no generation event found for execution ID: %s (found interaction types: %s)", executionID, strings.Join(interactionTypes, ", "))
	}
	logger.Infof("Found generation event for target: %s", getValue(genEvent.GenerateTarget))

	workflowRaw := extractWorkflow(eventsForExecution)
	workspaceID := extractWorkspaceID(eventsForExecution)

	s := fetchWorkspaceInfo(ctx, workspaceID, logger)

	// Determine output directory
	outputDir := flags.Directory
	if outputDir == "" {
		outputDir = filepath.Join("/tmp", fmt.Sprintf("%s.%s", s.org, s.workspace))
	}

	printGenerationDetails(logger, genEvent, s)

	if err := setupDirectoryStructure(outputDir, eventsForExecution, logger); err != nil {
		return err
	}

	speakeasyDir := filepath.Join(outputDir, ".speakeasy")

	_, skipSpecDownload, err := downloadSpec(ctx, genEvent, s, speakeasyDir, logger)
	if err != nil {
		return err
	}

	if err := writeGenConfig(genEvent, speakeasyDir, executionID); err != nil {
		return err
	}

	if err := writeWorkflowFiles(workflowRaw, speakeasyDir, skipSpecDownload, flags.UseRawWorkflow, executionID, logger); err != nil {
		return err
	}

	return finishAndRegenerate(outputDir, logger)
}

func fetchCLIEvents(ctx context.Context, executionID string) ([]shared.CliEvent, error) {
	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK client: %w", err)
	}

	logger := log.From(ctx)
	logger.Infof("Fetching CLI events for execution ID: %s", executionID)

	limit := int64(100)
	req := operations.SearchWorkspaceEventsRequest{
		ExecutionID: &executionID,
		Limit:       &limit,
	}

	res, err := s.Events.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CLI events: %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status code %d when searching for events", res.StatusCode)
	}

	if res.CliEventBatch == nil || len(res.CliEventBatch) == 0 {
		return nil, fmt.Errorf("no CLI events found for execution ID: %s", executionID)
	}

	return res.CliEventBatch, nil
}

func collectInteractionTypes(events []shared.CliEvent) []string {
	var interactionTypes []string
	for _, event := range events {
		interactionTypes = append(interactionTypes, string(event.InteractionType))
	}
	return interactionTypes
}

func findGenEvent(events []shared.CliEvent) *shared.CliEvent {
	for _, event := range events {
		if event.InteractionType == shared.InteractionTypeTargetGenerate && event.GenerateConfigPreRaw != nil && *event.GenerateConfigPreRaw != "" {
			return &event
		}
	}
	return nil
}

func extractWorkflow(events []shared.CliEvent) string {
	var workflowRaw *string

	for _, event := range events {
		if event.WorkflowPreRaw != nil && workflowRaw == nil && *event.WorkflowPreRaw != "" {
			workflowRaw = event.WorkflowPreRaw
		}
	}

	return *workflowRaw
}

func extractWorkspaceID(events []shared.CliEvent) string {
	for _, event := range events {
		if event.WorkspaceID != "" {
			return event.WorkspaceID
		}
	}
	return ""
}

func fetchWorkspaceInfo(ctx context.Context, workspaceID string, logger log.Logger) slugs {
	if workspaceID == "" {
		return slugs{}
	}

	s, err := core.GetSDKFromContext(ctx)
	if err != nil {
		logger.Warnf("Failed to get SDK client: %v", err)
		return slugs{}
	}

	wsReq := operations.GetWorkspaceRequest{
		WorkspaceID: &workspaceID,
	}
	wRes, err := s.Workspaces.GetByID(ctx, wsReq)
	if err != nil {
		logger.Warnf("Failed to fetch workspace info: %v", err)
		return slugs{}
	}

	if wRes.Workspace == nil {
		return slugs{}
	}

	result := slugs{workspace: wRes.Workspace.Slug}
	if wRes.Workspace.OrganizationID == "" {
		return result
	}

	orgReq := operations.GetOrganizationRequest{
		OrganizationID: wRes.Workspace.OrganizationID,
	}
	orgRes, err := s.Organizations.Get(ctx, orgReq)
	if err != nil {
		logger.Warnf("Failed to fetch organization info: %v", err)
		return result
	}

	if orgRes.Organization == nil {
		return result
	}

	result.org = orgRes.Organization.Slug
	return result
}

func setupDirectoryStructure(outputDir string, events []shared.CliEvent, logger log.Logger) error {
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

	speakeasyDir := filepath.Join(outputDir, ".speakeasy")
	if err := os.MkdirAll(speakeasyDir, 0755); err != nil {
		return fmt.Errorf("failed to create .speakeasy directory: %w", err)
	}

	logsDir := filepath.Join(speakeasyDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	eventsFile := filepath.Join(logsDir, "repro-cli-events.json")
	eventsJSON, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CLI events to JSON: %w", err)
	}
	if err := os.WriteFile(eventsFile, eventsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write CLI events to file: %w", err)
	}
	logger.Infof("Saved CLI events to %s", eventsFile)

	return nil
}

func downloadSpec(ctx context.Context, genEvent *shared.CliEvent, s slugs, speakeasyDir string, logger log.Logger) (string, bool, error) {
	if genEvent.SourceNamespaceName == nil || genEvent.SourceRevisionDigest == nil {
		logger.Warnf("Source namespace or revision digest missing - will use original workflow")
		return "", true, nil
	}

	// Download the merged/overlayed spec that was actually used for generation
	location := fmt.Sprintf("registry.speakeasyapi.dev/%s/%s/%s@%s", s.org, s.workspace, *genEvent.SourceNamespaceName, *genEvent.SourceRevisionDigest)
	d := workflow.Document{
		Location: workflow.LocationString(location),
	}

	tempLocation := workflow.GetTempDir()
	documentOut, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, tempLocation)
	if err != nil {
		return "", false, fmt.Errorf("failed to download spec: %w", err)
	}

	inputPath := filepath.Join(speakeasyDir, inputSpecFilename)
	if err := utils.CopyFile(documentOut.LocalFilePath, inputPath); err != nil {
		return "", false, fmt.Errorf("failed to copy spec to input location: %w", err)
	}

	return inputPath, false, nil
}

func writeGenConfig(genEvent *shared.CliEvent, speakeasyDir, executionID string) error {
	genConfig := genEvent.GenerateConfigPreRaw
	if genConfig == nil || *genConfig == "" {
		return fmt.Errorf("no gen.yaml found in any event for execution ID: %s", executionID)
	}

	genPath := filepath.Join(speakeasyDir, "gen.yaml")
	return os.WriteFile(genPath, []byte(*genConfig), 0644)
}

func writeWorkflowFiles(workflowRaw string, speakeasyDir string, skipSpecDownload, useRawWorkflow bool, executionID string, logger log.Logger) error {
	if workflowRaw == "" {
		return fmt.Errorf("no workflow found in any event for execution ID: %s", executionID)
	}

	workflowPath := filepath.Join(speakeasyDir, "workflow.original.yaml")
	if err := os.WriteFile(workflowPath, []byte(workflowRaw), 0644); err != nil {
		return fmt.Errorf("failed to write workflow.original.yaml: %w", err)
	}

	if skipSpecDownload || useRawWorkflow {
		workflowModPath := filepath.Join(speakeasyDir, "workflow.yaml")
		if err := os.WriteFile(workflowModPath, []byte(workflowRaw), 0644); err != nil {
			return fmt.Errorf("failed to write workflow.yaml: %w", err)
		}
		logger.Infof("Using original workflow (--use-raw-workflow enabled)")
		return nil
	}

	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(workflowRaw), &wf); err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	for sourceID, source := range wf.Sources {
		source.Inputs = []workflow.Document{{Location: workflow.LocationString(".speakeasy/" + inputSpecFilename)}}
		source.Overlays = nil
		source.Transformations = nil
		source.Output = nil
		source.Ruleset = nil
		source.Registry = nil
		wf.Sources[sourceID] = source
	}

	modifiedWorkflow, err := yaml.Marshal(&wf)
	if err != nil {
		return fmt.Errorf("failed to marshal modified workflow: %w", err)
	}

	workflowModPath := filepath.Join(speakeasyDir, "workflow.yaml")
	if err := os.WriteFile(workflowModPath, modifiedWorkflow, 0644); err != nil {
		return fmt.Errorf("failed to write workflow.yaml: %w", err)
	}
	logger.Infof("Modified workflow to use local merged/overlayed spec")

	return nil
}

func finishAndRegenerate(outputDir string, logger log.Logger) error {
	logger.Infof("\nRunning speakeasy run --output=console...")

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

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

func printGenerationDetails(logger log.Logger, genEvent *shared.CliEvent, s slugs) {
	logger.Infof("Generation Details:")

	printIfNotEmpty := func(condition bool, format string, args ...interface{}) {
		if condition {
			logger.Infof(format, args...)
		}
	}

	printIfNotEmpty(!genEvent.CreatedAt.IsZero(), "  Created At: %s", genEvent.CreatedAt.Format(time.RFC3339))
	printIfNotEmpty(s.org != "" && s.workspace != "", "  Workspace: https://app.speakeasy.com/org/%s/%s", s.org, s.workspace)
	printIfNotEmpty(genEvent.SpeakeasyAPIKeyName != "", "  API Key Name: %s", genEvent.SpeakeasyAPIKeyName)
	printIfNotEmpty(genEvent.SpeakeasyVersion != "", "  CLI Version: %s", genEvent.SpeakeasyVersion)
	printIfNotEmpty(genEvent.GenerateTarget != nil, "  Target: %s", getValue(genEvent.GenerateTarget))
	printIfNotEmpty(genEvent.GenerateTargetName != nil, "  Target Name: %s", getValue(genEvent.GenerateTargetName))
	printIfNotEmpty(genEvent.GenerateTargetVersion != nil, "  Target Version: %s", getValue(genEvent.GenerateTargetVersion))
	printIfNotEmpty(genEvent.GenerateRepoURL != nil && *genEvent.GenerateRepoURL != "", "  Repo URL: %s", getValue(genEvent.GenerateRepoURL))
	printIfNotEmpty(genEvent.GhActionRunLink != nil && *genEvent.GhActionRunLink != "", "  GitHub Action Run: %s", getValue(genEvent.GhActionRunLink))
	printIfNotEmpty(genEvent.GenerateGenLockID != nil, "  Gen Lock ID: %s", getValue(genEvent.GenerateGenLockID))

	printStatus(logger, genEvent)
	printIfNotEmpty(genEvent.SourceNamespaceName != nil, "  Source Namespace: %s", getValue(genEvent.SourceNamespaceName))
	printConfigStatus(logger, genEvent)
	printWorkflowStatus(logger, genEvent)
}

func printStatus(logger log.Logger, genEvent *shared.CliEvent) {
	switch {
	case genEvent.Success:
		logger.Infof("  Status: Success")
	case genEvent.Error != nil:
		logger.Infof("  Error: %s", *genEvent.Error)
	default:
		logger.Infof("  Status: Failed")
	}
}

func printConfigStatus(logger log.Logger, genEvent *shared.CliEvent) {
	hasPreConfig := genEvent.GenerateConfigPreRaw != nil
	hasPostConfig := genEvent.GenerateConfigPostRaw != nil

	configStatus := getConfigStatus(hasPreConfig, hasPostConfig)
	if configStatus != "" {
		logger.Infof("  Config: %s", configStatus)
	}
}

func getConfigStatus(hasPreConfig, hasPostConfig bool) string {
	switch {
	case !hasPreConfig && !hasPostConfig:
		return "gen.yaml not found"
	case hasPreConfig && hasPostConfig:
		return "gen.yaml (pre & post)"
	case hasPreConfig:
		return "gen.yaml (pre only)"
	case hasPostConfig:
		return "gen.yaml (post only)"
	default:
		return ""
	}
}

func printWorkflowStatus(logger log.Logger, genEvent *shared.CliEvent) {
	hasPreWorkflow := genEvent.WorkflowPreRaw != nil
	hasPostWorkflow := genEvent.WorkflowPostRaw != nil

	workflowStatus := getWorkflowStatus(hasPreWorkflow, hasPostWorkflow)
	if workflowStatus != "" {
		logger.Infof("  Workflow: %s", workflowStatus)
	}
}

func getWorkflowStatus(hasPreWorkflow, hasPostWorkflow bool) string {
	switch {
	case hasPreWorkflow && hasPostWorkflow:
		return "available (pre & post)"
	case hasPreWorkflow:
		return "available (pre only)"
	case hasPostWorkflow:
		return "available (post only)"
	default:
		return ""
	}
}
