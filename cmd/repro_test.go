package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/testutils"
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

func TestReproEndToEnd(t *testing.T) {
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
	}

	// Get the execution ID from the run output
	executionID := ""
	lines := strings.Split(runOutputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--execution-id") {
			t.Logf("Found execution ID: %s", line)
			executionID = strings.TrimSpace(strings.Split(line, "--execution-id ")[1])
			t.Logf("Execution ID: %s", executionID)
		}
	}
	if executionID == "" {
		for _, line := range lines {
			if strings.HasPrefix(line, "Speakeasy Reproduction ID:") {
				executionID = strings.TrimSpace(strings.Split(line, "Speakeasy Reproduction ID:")[1])
				t.Logf("Execution ID: %s", executionID)
			}
		}
	}

	if executionID == "" {
		t.Fatalf("No execution ID found in run output")
	}
	t.Logf("Execution ID: %s", executionID)

	// Now run repro
	t.Logf("Running repro command with execution ID: %s", executionID)
	t.Logf("Repro directory: %s", reproDir)
	reproCmd := exec.Command(speakeasyBinary, "repro", "--execution-id", executionID,
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
