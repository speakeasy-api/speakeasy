package integration_tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
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
			t.Parallel()
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
				require.NoError(t, err)
			}

			// Execute commands from the temporary directory
			os.Chdir(temp)
			err := workflowFile.Validate(generate.GetSupportedLanguages())
			require.NoError(t, err)
			err = os.MkdirAll(".speakeasy", 0o755)
			require.NoError(t, err)
			err = workflow.Save(".", workflowFile)
			require.NoError(t, err)
			args := []string{"run", "-t", "all", "--pinned"}
			if tt.withForce {
				args = append(args, "--force", "true")
			}

			cmdErr := execute(t, args...)
			require.NoError(t, cmdErr)

			if tt.withCodeSamples {
				codeSamplesPath := filepath.Join(tt.outdirs[0], "codeSamples.yaml")
				content, err := os.ReadFile(codeSamplesPath)
				require.NoError(t, err, "No readable file %s exists", codeSamplesPath)

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

func execute(t *testing.T, args ...string) error {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	mainGo := filepath.Join(baseFolder, "main.go")
	cmd := exec.Command("go", append([]string{"run", mainGo}, args...)...)
	cmd.Env = os.Environ()
	curWd, err := os.Getwd()
	require.NoError(t, err)
	cmd.Dir = curWd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
		{
			name: "test remote input (caused an issue in Windows)",
			inputDocs: []string{
				"https://petstore3.swagger.io/api/v3/openapi.yaml",
			},
			out: "output.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
					require.NoError(t, err)
				}
				inputs = append(inputs, workflow.Document{
					Location: inputDoc,
				})
			}
			var overlays []workflow.Overlay
			for _, overlay := range tt.overlays {
				if isLocalFileReference(overlay) {
					err := copyFile(fmt.Sprintf("resources/%s", overlay), fmt.Sprintf("%s/%s", temp, overlay))
					require.NoError(t, err)
				}
				overlays = append(overlays, workflow.Overlay{
					Document: &workflow.Document{
						Location: overlay,
					},
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
			require.NoError(t, err)
			err = os.MkdirAll(".speakeasy", 0o755)
			require.NoError(t, err)
			err = workflow.Save(".", workflowFile)
			require.NoError(t, err)
			args := []string{"run", "-s", "all", "--pinned"}
			cmdErr := execute(t, args...)
			require.NoError(t, cmdErr)

			content, err := os.ReadFile(tt.out)
			require.NoError(t, err, "No readable file %s exists", tt.out)

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

func TestFallbackCodeSamplesWorkflow(t *testing.T) {
	t.Parallel()
	spec := `{
		"openapi": "3.0.0",
		"info": {
		"title": "Swagger Petstore",
		"version": "1.0.0"
		},
		"servers": [
			{
				"url": "http://petstore.swagger.io/v1"
			}
		],
		"paths": {
		"/pets": {
			"post": {
			"requestBody": {
				"content": {
				"application/json": {
					"schema": {
					"type": "object",
					"properties": {
						"name": {
						"type": "string"
						},
						"breed": {
						"type": "string"
						}
					},
					"example": {
						"name": "doggie",
						"breed": "labrador"
					}
					}
				}
				}
			},
			"responses": {
				"200": {
				"description": "pet created"
				}
			}
			}
		}
		}
	}`
	// Write the spec to a temp file
	temp := setupTestDir(t)
	specPath := filepath.Join(temp, "spec.yaml")
	err := os.WriteFile(specPath, []byte(spec), 0o644)
	require.NoError(t, err)

	tempOutputFile := "./output.yaml"
	relFilePath, err := filepath.Rel(temp, specPath)
	require.NoError(t, err)
	// Create a workflow file, input is petstore, overlay is fallbackCodeSamples.yaml
	workflowFile := &workflow.Workflow{
		Version: workflow.WorkflowVersion,
		Sources: map[string]workflow.Source{
			"first-source": {
				Inputs: []workflow.Document{{Location: relFilePath}},
				Overlays: []workflow.Overlay{
					{
						FallbackCodeSamples: &workflow.FallbackCodeSamples{
							FallbackCodeSamplesLanguage: "shell",
						},
					},
				},
				Output: &tempOutputFile,
			},
		},
	}

	// Now run the workflow
	os.Chdir(temp)
	err = workflowFile.Validate(generate.GetSupportedLanguages())
	require.NoError(t, err)
	err = os.MkdirAll(".speakeasy", 0o755)
	require.NoError(t, err)
	err = workflow.Save(".", workflowFile)
	require.NoError(t, err)

	// Read the saved workflow file and print it for debugging
	rawWorkflow, err := os.ReadFile(filepath.Join(".speakeasy", "workflow.yaml"))
	require.NoError(t, err)
	fmt.Println(string(rawWorkflow))

	args := []string{"run", "-s", "all", "--pinned"}
	cmdErr := execute(t, args...)
	require.NoError(t, cmdErr)

	// List directory contents for debugging
	files, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, file := range files {
		fmt.Println(file.Name())
	}

	// Check that the output file contains the expected code samples
	content, err := os.ReadFile(tempOutputFile)
	require.NoError(t, err, "No readable file %s exists", tempOutputFile)
	fmt.Println(string(content))
	require.Contains(t, string(content), "curl")
	// Check it contains the example
	require.Contains(t, string(content), "doggie")

}
