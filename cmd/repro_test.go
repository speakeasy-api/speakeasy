package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReproEndToEnd(t *testing.T) {
	// Create test directories
	originalDir := "/tmp/speakeasy-test-repro/original"
	reproDir := "/tmp/speakeasy-test-repro/repro"
	speakeasyDir := filepath.Join(originalDir, ".speakeasy")

	// Clean up any existing test directories
	os.RemoveAll("/tmp/speakeasy-test-repro")
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

	// Build the speakeasy CLI
	buildCmd := exec.Command("go", "build", "-o", "/tmp/speakeasy-test", ".")
	buildCmd.Dir = "/Users/da/code/speakeasy-cli"
	buildOutput, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build speakeasy CLI: %s", string(buildOutput))

	// Run speakeasy run command in the original directory
	runCmd := exec.Command("/tmp/speakeasy-test", "run", "--output=console", "--pinned")
	runCmd.Dir = originalDir
	var runOutput bytes.Buffer
	runCmd.Stdout = &runOutput
	runCmd.Stderr = &runOutput

	err = runCmd.Run()
	runOutputStr := runOutput.String()
	t.Logf("Run command output:\n%s", runOutputStr)

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
		t.Fatalf("No execution ID found in run output")
	}
	t.Logf("Execution ID: %s", executionID)

	// Now run repro
	reproCmd := exec.Command("/tmp/speakeasy-test", "repro", "--execution-id", executionID,
		"--directory", reproDir)
	reproCmd.Dir = originalDir
	var reproOutput bytes.Buffer
	reproCmd.Stdout = &reproOutput
	reproCmd.Stderr = &reproOutput

	err = reproCmd.Run()
	reproOutputStr := reproOutput.String()
	t.Logf("Repro command output:\n%s", reproOutputStr)

	// Verify that the CLI events file was created
	eventsFile := filepath.Join(reproDir, ".speakeasy", "logs", "repro-cli-events.json")
	if _, err := os.Stat(eventsFile); os.IsNotExist(err) {
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
