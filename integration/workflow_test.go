package integration_tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/assert"
)

// These integration tests MUST be run in serial because we deal with changing working directories during the test.
// If running locally make sure you are running test functions individually TestGenerationWorkflows, TestSpecWorkflows, etc.
// If all test groups are run at the same time you will see test failures.

func TestGenerationWorkflows(t *testing.T) {
	tests := []struct {
		name            string
		targetTypes     []string
		outdirs         []string
		inputDoc        string
		withForce       bool
		withCodeSamples bool
	}{
		{
			name: "generation with remote document",
			targetTypes: []string{
				"go",
			},
			outdirs: []string{
				"go",
			},
			inputDoc: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml",
		},
		{
			name: "multi-target generation with local document",
			targetTypes: []string{
				"go",
				"typescript",
			},
			outdirs: []string{
				"go",
				"ts",
			},
			inputDoc: "spec.yaml",
		},
		{
			name: "code samples with json output",
			targetTypes: []string{
				"go",
			},
			outdirs: []string{
				"go",
			},
			inputDoc:        "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.json",
			withCodeSamples: true,
		},
		{
			name: "code samples with force",
			targetTypes: []string{
				"go",
			},
			outdirs: []string{
				"go",
			},
			inputDoc:        "spec.yaml",
			withCodeSamples: true,
			withForce:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := setupTestDir(t)

			// Create workflow file and associated resources
			workflowFile := &workflow.Workflow{
				Version: workflow.WorkflowVersion,
				Sources: make(map[string]workflow.Source),
				Targets: make(map[string]workflow.Target),
			}
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs: []workflow.Document{
					{
						Location: tt.inputDoc,
					},
				},
			}

			for i := range tt.targetTypes {
				target := workflow.Target{
					Target: tt.targetTypes[i],
					Source: "first-source",
					Output: &tt.outdirs[i],
				}
				if tt.withCodeSamples {
					target.CodeSamples = &workflow.CodeSamples{
						Output: "codeSamples.yaml",
					}
				}
				workflowFile.Targets[fmt.Sprintf("%d-target", i)] = target
			}

			if isLocalFileReference(tt.inputDoc) {
				err := copyFile("resources/spec.yaml", fmt.Sprintf("%s/%s", temp, tt.inputDoc))
				assert.NoError(t, err)
			}

			// Execute commands from the temporary directory
			os.Chdir(temp)
			err := workflowFile.Validate(generate.GetSupportedLanguages())
			assert.NoError(t, err)
			err = os.MkdirAll(".speakeasy", 0o755)
			assert.NoError(t, err)
			err = workflow.Save(".", workflowFile)
			assert.NoError(t, err)
			args := []string{"run", "-t", "all"}
			if tt.withForce {
				args = append(args, "--force", "true")
			}
			rootCmd.SetArgs(args)
			cmdErr := rootCmd.Execute()
			assert.NoError(t, cmdErr)

			if tt.withCodeSamples {
				codeSamplesPath := filepath.Join(tt.outdirs[0], "codeSamples.yaml")
				content, err := os.ReadFile(codeSamplesPath)
				assert.NoError(t, err, "No readable file %s exists", codeSamplesPath)

				if !strings.Contains(string(content), "update") {
					t.Errorf("Update actions do not exist in the codeSamples file")
				}
			}

			for i, targetType := range tt.targetTypes {
				checkForExpectedFiles(t, tt.outdirs[i], expectedFilesByLanguage(targetType))
			}
		})
	}
}

func TestSpecWorkflows(t *testing.T) {
	tests := []struct {
		name          string
		inputDocs     []string
		overlays      []string
		out           string
		expectedPaths []string
	}{
		{
			name: "overlay with local document",
			inputDocs: []string{
				"spec.yaml",
			},
			overlays: []string{
				"codeSamples.yaml",
			},
			out: "output.yaml",
		},
		{
			name: "overlay with json document",
			inputDocs: []string{
				"https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.json",
			},
			overlays: []string{
				"codeSamples-JSON.yaml",
			},
			out: "output.json",
		},
		{
			name: "test merging documents",
			inputDocs: []string{
				"part1.yaml",
				"part2.yaml",
			},
			out: "output.yaml",
			expectedPaths: []string{
				"/pet/findByStatus",
				"/store/inventory",
				"/user/login",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := setupTestDir(t)

			// Create workflow file and associated resources
			workflowFile := &workflow.Workflow{
				Version: workflow.WorkflowVersion,
				Sources: make(map[string]workflow.Source),
				Targets: make(map[string]workflow.Target),
			}

			var inputs []workflow.Document
			for _, inputDoc := range tt.inputDocs {
				if isLocalFileReference(inputDoc) {
					err := copyFile(fmt.Sprintf("resources/%s", inputDoc), fmt.Sprintf("%s/%s", temp, inputDoc))
					assert.NoError(t, err)
				}
				inputs = append(inputs, workflow.Document{
					Location: inputDoc,
				})
			}
			var overlays []workflow.Document
			for _, overlay := range tt.overlays {
				if isLocalFileReference(overlay) {
					err := copyFile(fmt.Sprintf("resources/%s", overlay), fmt.Sprintf("%s/%s", temp, overlay))
					assert.NoError(t, err)
				}
				overlays = append(overlays, workflow.Document{
					Location: overlay,
				})
			}
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs:   inputs,
				Overlays: overlays,
				Output:   &tt.out,
			}

			// Execute commands from the temporary directory
			os.Chdir(temp)
			err := workflowFile.Validate(generate.GetSupportedLanguages())
			assert.NoError(t, err)
			err = os.MkdirAll(".speakeasy", 0o755)
			assert.NoError(t, err)
			err = workflow.Save(".", workflowFile)
			assert.NoError(t, err)
			args := []string{"run", "-s", "all"}
			rootCmd.SetArgs(args)
			cmdErr := rootCmd.Execute()
			assert.NoError(t, cmdErr)

			content, err := os.ReadFile(tt.out)
			assert.NoError(t, err, "No readable file %s exists", tt.out)

			if len(tt.overlays) > 0 {
				if !strings.Contains(string(content), "x-codeSamples") {
					t.Errorf("overlay not successfully applied to output document")
				}
			}

			if len(tt.expectedPaths) > 0 {
				for _, path := range tt.expectedPaths {
					if !strings.Contains(string(content), path) {
						t.Errorf("Expected path %s not found in output document", path)
					}
				}
			}
		})
	}
}
