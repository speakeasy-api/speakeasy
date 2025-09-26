package run

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

func RunDependent(ctx context.Context, source, dependent string, flagsString string) error {
	if strings.Contains(flagsString, "dependent") {
		return fmt.Errorf("dependent flag must be removed")
	}

	if strings.Contains(flagsString, "source") {
		return fmt.Errorf("source flag must be removed")
	}

	if source == "" {
		return fmt.Errorf("source must be specified")
	}

	wf, projectDir, err := utils.GetWorkflowAndDir()
	if err != nil {
		return err
	}

	if wf.Sources[source].Output == nil {
		return fmt.Errorf("source %s must have an output location specified", source)
	}

	log.From(ctx).Infof("\n=== Building source %s ===\n", source)

	// Build source
	if err := runSpeakeasyFromLocation(ctx, ".", "run", "--source "+source); err != nil {
		return fmt.Errorf("failed to build source %s: %w", source, err)
	}

	sourceLocation := filepath.Join(projectDir, *wf.Sources[source].Output)
	sourceLocation, err = filepath.Abs(sourceLocation)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for source output: %w", err)
	}

	// Run dependents
	dependents := []string{dependent}
	if dependent == "all" {
		dependents = slices.Collect(maps.Keys(wf.Dependents))
	}

	for _, dependent := range dependents {
		r, ok := wf.Dependents[dependent]
		if !ok {
			return fmt.Errorf("dependent %s not found", dependent)
		}

		log.From(ctx).Infof("\n\n=== Rebuilding SDK %s ===\n", dependent)

		if err := processDependent(ctx, dependent, r, flagsString, sourceLocation, projectDir); err != nil {
			return fmt.Errorf("failed to process dependent %s: %w", dependent, err)
		}
	}

	return nil
}

func processDependent(ctx context.Context, dependentName string, dependent workflow.Dependent, flagsString string, sourceLocation string, projectDir string) error {
	logger := log.From(ctx)

	location := dependent.Location
	if location == "" {
		return fmt.Errorf("dependent %s has no location specified", dependentName)
	}

	// Resolve location relative to projectDir
	location = filepath.Join(projectDir, location)

	// Check if location exists
	if _, err := os.Stat(location); os.IsNotExist(err) {
		cloneCommand := dependent.CloneCommand
		if cloneCommand == "" {
			logger.Printf("Location %s does not exist and no clone command specified for dependent %s", location, dependentName)
			return nil
		}

		logger.Printf("Location %s does not exist for dependent %s", location, dependentName)

		// Ask user if they want to clone the repository to the specified location
		prompt := fmt.Sprintf("Would you like to clone the repository to %s?", location)
		if !interactivity.SimpleConfirm(prompt, true) {
			logger.Printf("\n🚫 Repository clone declined.")
			logger.Printf("💡 You can override the dependent location by creating a local workflow configuration file.")
			
			// Ask if they want to create workflow.local.yaml for local overrides
			localWorkflowPath := filepath.Join(projectDir, ".speakeasy", "workflow.local.yaml")
			prompt := fmt.Sprintf("Would you like to create %s to customize dependent locations?", localWorkflowPath)
			if interactivity.SimpleConfirm(prompt, true) {
				if err := CreateWorkflowLocalFile(projectDir); err != nil {
					logger.Printf("❌ Failed to create workflow.local.yaml: %v", err)
				} else {
					logger.Printf("✅ Created %s", localWorkflowPath)
					logger.Printf("📝 You can now uncomment and modify the dependents section to set custom locations.")
				}
			}
			
			return fmt.Errorf("location %s does not exist and user declined to clone", location)
		}

		// Clone the repository
		if err := cloneRepository(ctx, cloneCommand, location); err != nil {
			return fmt.Errorf("failed to run %s for %s: %w", cloneCommand, location, err)
		}

		logger.Printf("Successfully ran %s => %s", cloneCommand, location)
	}

	flagsString = fmt.Sprintf("--source-location %s %s", sourceLocation, flagsString)

	// Run speakeasy run from the location
	logger.Printf("Running speakeasy run from %s with flags: %s", location, flagsString)

	if err := runSpeakeasyFromLocation(ctx, location, "run", flagsString); err != nil {
		return fmt.Errorf("failed to run speakeasy from %s: %w", location, err)
	}

	return nil
}

func cloneRepository(ctx context.Context, cloneCommand, location string) error {
	logger := log.From(ctx)

	// Ensure the parent directory exists
	parentDir := filepath.Dir(location)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	logger.Printf("Running `%s %s`", cloneCommand, location)

	// Parse the clone command into command and arguments
	cmdParts := strings.Fields(cloneCommand)
	if len(cmdParts) == 0 {
		return fmt.Errorf("empty clone command")
	}

	// Add the location as the final argument
	cmdParts = append(cmdParts, location)

	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	return nil
}

func runSpeakeasyFromLocation(ctx context.Context, location, command, flagsString string) error {
	// Get the current executable path to run speakeasy from the same binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Parse flags string into arguments
	var args []string
	if flagsString != "" {
		args = strings.Fields(flagsString)
	}

	// Prepend command
	args = append([]string{command}, args...)

	cmd := exec.Command(execPath, args...)
	cmd.Dir = location
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func CreateWorkflowLocalFile(workflowDir string) error {
	workflowPath := filepath.Join(workflowDir, ".speakeasy", "workflow.yaml")
	localWorkflowPath := filepath.Join(workflowDir, ".speakeasy", "workflow.local.yaml")
	
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return fmt.Errorf("workflow.yaml file not found at %s", workflowPath)
	}
	
	if _, err := os.Stat(localWorkflowPath); err == nil {
		return fmt.Errorf("workflow.local.yaml already exists at %s", localWorkflowPath)
	}
	
	workflowContent, err := os.ReadFile(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to read workflow.yaml: %w", err)
	}
	
	commentedContent := commentOutYAMLContent(string(workflowContent))
	
	instructions := `# Local Workflow Configuration Override File
# 
# This file allows you to override any field from workflow.yaml for local development.
# Uncomment and modify any section below to override the corresponding values.
# 
# Only uncomment the specific fields (and their parent keys) that you want to override - you don't need to 
# uncomment entire sections if you only want to change one value.
#
# Example: To override just the speakeasyVersion, uncomment only that line:
# speakeasyVersion: "1.234.0"

`
	
	finalContent := instructions + commentedContent
	
	if err := os.WriteFile(localWorkflowPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write workflow.local.yaml: %w", err)
	}
	
	return nil
}

func commentOutYAMLContent(content string) string {
	lines := strings.Split(content, "\n")
	var commentedLines []string
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			commentedLines = append(commentedLines, line)
		} else {
			commentedLines = append(commentedLines, "# "+line)
		}
	}
	
	return strings.Join(commentedLines, "\n")
}
