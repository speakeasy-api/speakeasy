package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getReproDir() string {
	return filepath.Join(testutils.GetTempDir(), "speakeasy-test-repro")
}

func getOriginalDir() string {
	return filepath.Join(getReproDir(), "original")
}

func getReproSubDir() string {
	return filepath.Join(getReproDir(), "repro")
}

func getSpeakeasyBinary() string {
	return filepath.Join(testutils.GetTempDir(), "speakeasy-test")
}

func TestParseReproTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		expectedOrg string
		expectedWs  string
		expectedID  string
		expectError bool
	}{
		{
			name:        "Valid target",
			target:      "myorg_myworkspace_c303282d-f2e6-46ca-a04a-35d3d873712d",
			expectedOrg: "myorg",
			expectedWs:  "myworkspace",
			expectedID:  "c303282d-f2e6-46ca-a04a-35d3d873712d",
			expectError: false,
		},
		{
			name:        "Valid target with hyphens in org/workspace",
			target:      "my-org_my-workspace_c303282d-f2e6-46ca-a04a-35d3d873712d",
			expectedOrg: "my-org",
			expectedWs:  "my-workspace",
			expectedID:  "c303282d-f2e6-46ca-a04a-35d3d873712d",
			expectError: false,
		},
		{
			name:        "Invalid - missing execution ID",
			target:      "myorg_myworkspace",
			expectError: true,
		},
		{
			name:        "Invalid - malformed UUID",
			target:      "myorg_myworkspace_not-a-uuid",
			expectError: true,
		},
		{
			name:        "Invalid - empty string",
			target:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgSlug, workspaceSlug, executionID, err := parseReproTarget(tt.target)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOrg, orgSlug)
				assert.Equal(t, tt.expectedWs, workspaceSlug)
				assert.Equal(t, tt.expectedID, executionID)
			}
		})
	}
}

func TestReproEndToEnd(t *testing.T) {
	// For now skip on windows - building the temp binary is not working on windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping repro test on Windows")
		return
	}

	// Create test directories
	originalDir := getOriginalDir()
	reproDir := getReproSubDir()
	speakeasyDir := filepath.Join(originalDir, ".speakeasy")

	// Clean up any existing test directories
	os.RemoveAll(getReproDir())
	_ = reproDir // Will be used in a full implementation

	// Create the directory structure
	err := os.MkdirAll(speakeasyDir, 0755)
	require.NoError(t, err, "Failed to create .speakeasy directory")

	// Write gen.yaml
	genYAML := `configVersion: 2.0.0
generation:
  sdkClassName: petstore
typescript:
  enableMCPServer: true
  imports:
    option: openapi
    paths:
      callbacks: models/callbacks
      errors: models/errors
      operations: models/operations
      shared: models/components
      webhooks: models/webhooks
  packageName: 'petstore'
  templateVersion: v2
`
	err = os.WriteFile(filepath.Join(speakeasyDir, "gen.yaml"), []byte(genYAML), 0644)
	require.NoError(t, err, "Failed to write gen.yaml")

	// Write workflow.yaml
	workflowYAML := `workflowVersion: 1.0.0
speakeasyVersion: latest
sources:
    petstore-example-source:
        inputs:
            - location: .speakeasy/openapi.yaml
        registry:
            location: registry.speakeasyapi.dev/david-speakeasy/david-speakeasy/petstore-example
targets:
    petstore-example:
        target: typescript
        source: petstore-example-source
`
	err = os.WriteFile(filepath.Join(speakeasyDir, "workflow.yaml"), []byte(workflowYAML), 0644)
	require.NoError(t, err, "Failed to write workflow.yaml")

	// Create a minimal petstore OpenAPI spec for testing
	openapiContent := []byte(`openapi: 3.0.0
info:
  title: Petstore API
  version: 1.0.0
servers:
  - url: https://petstore.example.com/v1
security:
  - petstore_auth: []
components:
  securitySchemes:
    petstore_auth:
      type: http
      scheme: custom
      x-speakeasy-custom-security-scheme:
        schema:
          type: object
          properties:
            a:
              type: object
              properties:
                b:
                  type: string
paths:    
  /pets:
    get:
      summary: List all pets
      operationId: listPets
      parameters:
        - name: limit
          in: query
          description: How many items to return at one time (max 100)
          required: false
          schema:
            type: integer
            format: int32
      responses:
        '200':
          description: A paged array of pets
          content:
            application/json:    
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: integer
                      format: int64
                    name:
                      type: string
                    tag:
                      type: string
`)
	err = os.WriteFile(filepath.Join(speakeasyDir, "openapi.yaml"), openapiContent, 0644)
	require.NoError(t, err, "Failed to write openapi.yaml")

	// Build the speakeasy CLI binary
	speakeasyBinary := testutils.BuildTempBinary(t, getSpeakeasyBinary())

	// Run speakeasy run command in the original directory
	t.Logf("Running speakeasy from: %s", originalDir)
	t.Logf("Speakeasy binary: %s", speakeasyBinary)
	runCmd := exec.Command(speakeasyBinary, "run", "--output=console", "--pinned")
	runCmd.Dir = originalDir
	var runOutput bytes.Buffer
	runCmd.Stdout = &runOutput
	runCmd.Stderr = &runOutput

	err = runCmd.Run()
	runOutputStr := runOutput.String()
	t.Logf("Run command output:\n%s", runOutputStr)
	if err != nil {
		t.Logf("Run command failed with error: %v", err)
		// For tests, we can still continue even if the run failed
		// as we're testing the repro command itself
	}

	// Get the repro target from the run output
	reproTarget := ""
	executionID := ""
	lines := strings.Split(runOutputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "speakeasy repro ") {
			t.Logf("Found repro command: %s", line)
			if strings.Contains(line, "--execution-id") {
				// Old format: speakeasy repro --execution-id <id>
				executionID = strings.TrimSpace(strings.Split(line, "--execution-id ")[1])
				t.Logf("Found execution ID (old format): %s", executionID)
			} else {
				// New format: speakeasy repro <org>-<workspace>-<id>
				parts := strings.Split(line, "speakeasy repro ")
				if len(parts) >= 2 {
					reproTarget = strings.TrimSpace(parts[1])
					// Check if it's the placeholder format
					if strings.HasPrefix(reproTarget, "{org-slug}_{workspace-slug}_") {
						// Extract execution ID from placeholder format
						executionID = strings.TrimPrefix(reproTarget, "{org-slug}_{workspace-slug}_")
						t.Logf("Found execution ID (placeholder format): %s", executionID)
					} else {
						t.Logf("Found repro target (new format): %s", reproTarget)
					}
				}
			}
		}
	}

	// Determine what to use for repro command
	if reproTarget == "" && executionID != "" {
		// Use a dummy org/workspace for testing when we only have execution ID
		reproTarget = fmt.Sprintf("test-org_test-workspace_%s", executionID)
		t.Logf("Using synthetic repro target: %s", reproTarget)
	} else if reproTarget == "" {
		// For testing purposes, we'll use a dummy execution ID if we can't find one
		// This can happen when telemetry is disabled (speakeasy-self)
		t.Logf("Warning: No repro target or execution ID found in output, using dummy ID for test")
		reproTarget = "test-org_test-workspace_00000000-0000-0000-0000-000000000000"
	}

	// Now run repro
	t.Logf("Running repro command with target: %s", reproTarget)
	t.Logf("Repro directory: %s", reproDir)
	reproCmd := exec.Command(speakeasyBinary, "repro", reproTarget,
		"--directory", reproDir)
	reproCmd.Dir = originalDir
	var reproOutput bytes.Buffer
	reproCmd.Stdout = &reproOutput
	reproCmd.Stderr = &reproOutput

	err = reproCmd.Run()
	reproOutputStr := reproOutput.String()
	t.Logf("Repro command output:\n%s", reproOutputStr)
	if err != nil {
		t.Logf("Repro command failed with error: %v", err)
	}

	// Verify that the CLI events file was created
	eventsFile := filepath.Join(reproDir, ".speakeasy", "logs", "repro-cli-events.json")
	t.Logf("Checking for events file at: %s", eventsFile)

	// List contents of repro directory for debugging
	if entries, err := os.ReadDir(reproDir); err == nil {
		t.Logf("Contents of repro directory:")
		for _, entry := range entries {
			t.Logf("  - %s (dir: %v)", entry.Name(), entry.IsDir())
		}
	}

	if _, err := os.Stat(eventsFile); os.IsNotExist(err) {
		// Check if .speakeasy/logs exists
		logsDir := filepath.Join(reproDir, ".speakeasy", "logs")
		if entries, err := os.ReadDir(logsDir); err == nil {
			t.Logf("Contents of logs directory:")
			for _, entry := range entries {
				t.Logf("  - %s", entry.Name())
			}
		} else {
			t.Logf("Could not read logs directory: %v", err)
		}
		t.Fatalf("CLI events file not found at %s", eventsFile)
	}

	// Read and verify the events file
	eventsData, err := os.ReadFile(eventsFile)
	if err != nil {
		t.Fatalf("Failed to read CLI events file: %v", err)
	}

	var events []interface{}
	if err := json.Unmarshal(eventsData, &events); err != nil {
		t.Fatalf("Failed to parse CLI events JSON: %v", err)
	}

	if len(events) == 0 {
		t.Fatalf("CLI events file is empty")
	}

	t.Logf("Found %d CLI events saved to %s", len(events), eventsFile)
}
