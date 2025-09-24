package integration_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/speakeasy-api/speakeasy/cmd"
	"github.com/speakeasy-api/versioning-reports/versioning"
	"github.com/spf13/cobra"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

// These integration tests MUST be run in serial because we deal with changing working directories during the test.
// If running locally make sure you are running test functions individually TestGenerationWorkflows, TestSpecWorkflows, etc.
// If all test groups are run at the same time you will see test failures.

func TestWorkflowWithEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		location workflow.LocationString
		want     string
	}{
		{
			name: "simple substitution",
			env: map[string]string{
				"MY_FILE_FULL_PATH": "spec.yaml",
			},
			location: workflow.LocationString("${MY_FILE_FULL_PATH}"),
			want:     "spec.yaml",
		},
		{
			name: "fallback substitution",
			env: map[string]string{
				"MY_FILE_PATH": "",
				"MY_FILE_EXT":  "",
			},
			location: workflow.LocationString("${MY_FILE_PATH:-spec}.${MY_FILE_EXT:-yaml}"),
			want:     "spec.yaml",
		},
		{
			name: "colon plus substitution",
			env: map[string]string{
				"MY_FILE_APPEND": "enabled",
			},
			location: workflow.LocationString("spec${MY_FILE_APPEND:+.yaml}"),
			want:     "spec.yaml",
		},
		{
			name: "multiple substitutions",
			env: map[string]string{
				"MY_FILE_DIR":  "specs",
				"MY_FILE_NAME": "spec",
				"MY_FILE_EXT":  "yaml",
			},
			location: workflow.LocationString("${MY_FILE_DIR}/${MY_FILE_NAME}.${MY_FILE_EXT}"),
			want:     "specs/spec.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temp := setupTestDir(t)

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			resolved := tt.location.Resolve()
			require.Equal(t, tt.want, resolved)

			workflowFile := &workflow.Workflow{
				Version: workflow.WorkflowVersion,
				Sources: make(map[string]workflow.Source),
				Targets: make(map[string]workflow.Target),
			}
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs: []workflow.Document{
					{Location: tt.location},
				},
			}
			workflowFile.Targets["test-target"] = workflow.Target{
				Target: "typescript",
				Source: "first-source",
			}

			require.NoError(t, os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755))
			require.NoError(t, workflow.Save(temp, workflowFile))

			destPath := filepath.Join(temp, resolved)
			require.NoError(t, os.MkdirAll(filepath.Dir(destPath), 0o755))
			require.NoError(t, copyFile(filepath.Join("resources", "spec.yaml"), destPath))

			require.NoError(t, execute(t, temp, "run", "-t", "all", "--pinned", "--skip-compile").Run())

			_, err := os.Stat(filepath.Join(temp, "README.md"))
			require.NoError(t, err)
		})
	}
}

func TestGenerationWorkflows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		targetTypes     []string
		outdirs         []string
		inputDoc        string
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
			inputDoc: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.json",
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
			inputDoc:        "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.json",
			withCodeSamples: true,
		},
		{
			name: "code samples",
			targetTypes: []string{
				"go",
			},
			outdirs: []string{
				"go",
			},
			inputDoc:        "spec.yaml",
			withCodeSamples: true,
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
						Location: workflow.LocationString(tt.inputDoc),
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

			err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
			require.NoError(t, err)
			err = workflow.Save(temp, workflowFile)
			require.NoError(t, err)
			args := []string{"run", "-t", "all", "--pinned", "--skip-compile"}

			cmdErr := execute(t, temp, args...).Run()
			require.NoError(t, cmdErr)

			if tt.withCodeSamples {
				codeSamplesPath := filepath.Join(temp, tt.outdirs[0], "codeSamples.yaml")
				content, err := os.ReadFile(codeSamplesPath)
				require.NoError(t, err, "No readable file %s exists", codeSamplesPath)

				if !strings.Contains(string(content), "update") {
					t.Errorf("Update actions do not exist in the codeSamples file")
				}
			}

			for i, targetType := range tt.targetTypes {
				checkForExpectedFiles(t, filepath.Join(temp, tt.outdirs[i]), expectedFilesByLanguage(targetType))
			}
		})
	}
}

func TestInputOnlyWorkflow(t *testing.T) {
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
				Location: "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.json",
			},
		},
	}

	err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)
	args := []string{"run", "-s", "first-source", "--pinned", "--skip-compile"}
	cmdErr := execute(t, temp, args...).Run()
	require.NoError(t, cmdErr)

	args = []string{"run", "-s", "all", "--pinned", "--skip-compile"}
	cmdErr = execute(t, temp, args...).Run()
	require.NoError(t, cmdErr)
}

type Runnable interface {
	Run() error
}

type subprocessRunner struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

func (r *subprocessRunner) Run() error {
	err := r.cmd.Run()
	if err != nil {
		fmt.Println(r.out.String())
		return err
	}
	return nil
}

func execute(t *testing.T, wd string, args ...string) Runnable {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	baseFolder := filepath.Join(filepath.Dir(filename), "..")
	mainGo := filepath.Join(baseFolder, "main.go")
	execCmd := exec.Command("go", append([]string{"run", mainGo}, args...)...)
	execCmd.Env = os.Environ()
	execCmd.Dir = wd

	// store stdout and stderr in a buffer and output it all in one go if there's a failure
	out := bytes.Buffer{}
	execCmd.Stdout = &out
	execCmd.Stderr = &out

	return &subprocessRunner{
		cmd: execCmd,
		out: &out,
	}
}

// executeI is a helper function to execute the main.go file inline. It can help when debugging integration tests
// We should not use it on multiple tests at once as they will share memory: this can create issues.
// so we leave it around as a little helper method: swap out execute for executeI and debug breakpoints work
var mutex sync.Mutex
var rootCmd = cmd.CmdForTest(version, artifactArch)

func executeI(t *testing.T, wd string, args ...string) Runnable {
	mutex.Lock()
	t.Helper()
	rootCmd.SetArgs(args)
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(wd))

	return &cmdRunner{
		rootCmd: rootCmd,
		cleanup: func() {
			require.NoError(t, os.Chdir(oldWD))
			mutex.Unlock()
		},
	}
}

type cmdRunner struct {
	rootCmd *cobra.Command
	cleanup func()
}

func (c *cmdRunner) Run() error {
	defer c.cleanup()
	return c.rootCmd.Execute()
}

func TestSpecWorkflows(t *testing.T) {
	tests := []struct {
		name            string
		inputDocs       []string
		overlays        []string
		transformations []workflow.Transformation
		out             string
		expectedPaths   []string
		unexpectedPaths []string
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
				"https://raw.githubusercontent.com/OAI/OpenAPI-Specification/refs/tags/3.1.0/examples/v3.0/petstore.json",
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
		{
			name:      "test simple transformation",
			inputDocs: []string{"spec.yaml"},
			transformations: []workflow.Transformation{
				{
					FilterOperations: &workflow.FilterOperationsOptions{
						Operations: "findPetsByTags",
					},
				},
			},
			out: "output.yaml",
			expectedPaths: []string{
				"/pet/findByTags",
			},
			unexpectedPaths: []string{
				"/pet/findByStatus",
			},
		},
		{
			name:      "test merge with transformation",
			inputDocs: []string{"part1.yaml", "part2.yaml"},
			transformations: []workflow.Transformation{
				{
					FilterOperations: &workflow.FilterOperationsOptions{
						Operations: "getInventory",
					},
				},
			},
			out: "output.yaml",
			expectedPaths: []string{
				"/store/inventory",
			},
			unexpectedPaths: []string{
				"/store/order",
			},
		},
		{
			name:      "test overlay with transformation",
			inputDocs: []string{"spec.yaml"},
			overlays:  []string{"renameOperationOverlay.yaml"},
			transformations: []workflow.Transformation{
				{
					FilterOperations: &workflow.FilterOperationsOptions{
						Operations: "findByTagsNew",
					},
				},
			},
			out: "output.yaml",
			expectedPaths: []string{
				"/pet/findByTags",
			},
			unexpectedPaths: []string{
				"/pet/findByStatus",
			},
		},
		{
			name:      "test merge, overlay, and transformation",
			inputDocs: []string{"part1.yaml", "part2.yaml"},
			overlays:  []string{"renameOperationOverlay.yaml"},
			transformations: []workflow.Transformation{
				{
					FilterOperations: &workflow.FilterOperationsOptions{
						Operations: "findByTagsNew",
					},
				},
			},
			out: "output.yaml",
			expectedPaths: []string{
				"/pet/findByTags",
			},
			unexpectedPaths: []string{
				"/pet/findByStatus",
			},
		},
		{
			name:      "test simple json conversion",
			inputDocs: []string{"part1.yaml"},
			out:       "output.json",
			expectedPaths: []string{
				"/pet/findByTags",
			},
		},
		{
			name:      "test json conversion with overlay",
			inputDocs: []string{"part1.yaml"},
			overlays:  []string{"renameOperationOverlay.yaml"},
			out:       "output.json",
			expectedPaths: []string{
				"/pet/findByTags",
			},
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
					inputDoc = filepath.Join(temp, inputDoc)
				}
				inputs = append(inputs, workflow.Document{
					Location: workflow.LocationString(inputDoc),
				})
			}
			var overlays []workflow.Overlay
			for _, overlay := range tt.overlays {
				if isLocalFileReference(overlay) {
					err := copyFile(fmt.Sprintf("resources/%s", overlay), fmt.Sprintf("%s/%s", temp, overlay))
					require.NoError(t, err)
					overlay = filepath.Join(temp, overlay)
				}
				overlays = append(overlays, workflow.Overlay{
					Document: &workflow.Document{
						Location: workflow.LocationString(overlay),
					},
				})
			}

			outputFull := filepath.Join(temp, tt.out)
			workflowFile.Sources["first-source"] = workflow.Source{
				Inputs:          inputs,
				Overlays:        overlays,
				Transformations: tt.transformations,
				Output:          &outputFull,
			}

			err := os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
			// Ignore error if directory already exists
			if err != nil && !os.IsExist(err) {
				require.NoError(t, err)
			}
			err = workflow.Save(temp, workflowFile)
			require.NoError(t, err)
			args := []string{"run", "-s", "all", "--pinned", "--skip-compile"}

			cmdErr := execute(t, temp, args...).Run()
			require.NoError(t, cmdErr)

			content, err := os.ReadFile(filepath.Join(temp, tt.out))
			require.NoError(t, err, "No readable file %s exists", filepath.Join(temp, tt.out))

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

			if len(tt.unexpectedPaths) > 0 {
				for _, path := range tt.unexpectedPaths {
					if strings.Contains(string(content), path) {
						t.Errorf("Unexpected path %s found in output document", path)
					}
				}
			}

			if !utils.HasYAMLExt(tt.out) {
				require.True(t, json.Valid(content))
			}
		})
	}
}

func TestFallbackCodeSamplesWorkflow(t *testing.T) {
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
				Inputs: []workflow.Document{{Location: workflow.LocationString(relFilePath)}},
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
	err = os.MkdirAll(filepath.Join(temp, ".speakeasy"), 0o755)
	require.NoError(t, err)
	err = workflow.Save(temp, workflowFile)
	require.NoError(t, err)

	// Read the saved workflow file and print it for debugging
	rawWorkflow, err := os.ReadFile(filepath.Join(temp, ".speakeasy", "workflow.yaml"))
	require.NoError(t, err)
	fmt.Println(string(rawWorkflow))

	args := []string{"run", "-s", "all", "--pinned"}
	reports, _, cmdErr := versioning.WithVersionReportCapture[bool](context.Background(), func(ctx context.Context) (bool, error) {
		err := execute(t, temp, args...).Run()
		return true, err
	})
	require.NoError(t, cmdErr)
	require.NotNil(t, reports)
	require.True(t, len(reports.Reports) > 0, "must have version reports")
	require.Truef(t, reports.MustGenerate(), "must have gen.lock")

	require.NoError(t, cmdErr)

	// List directory contents for debugging
	files, err := os.ReadDir(temp)
	require.NoError(t, err)
	for _, file := range files {
		fmt.Println(file.Name())
	}

	// Check that the output file contains the expected code samples
	content, err := os.ReadFile(filepath.Join(temp, tempOutputFile))
	require.NoError(t, err, "No readable file %s exists", tempOutputFile)
	fmt.Println(string(content))
	require.Contains(t, string(content), "curl")
	// Check it contains the example
	require.Contains(t, string(content), "doggie")
}
