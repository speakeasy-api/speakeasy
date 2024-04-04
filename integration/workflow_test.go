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

func TestCodeSampleWorkflows(t *testing.T) {
	tests := []struct {
		name       string
		targetType string
		outdir     string
		inputDoc   string
		withForce  bool
	}{
		{
			name:       "codeSamples with remote document",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml",
		},
		{
			name:       "codeSamples with local document",
			targetType: "typescript",
			outdir:     "ts",
			inputDoc:   "spec.yaml",
		},
		{
			name:       "codeSamples with json output",
			targetType: "typescript",
			outdir:     "ts",
			inputDoc:   "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.json",
		},
		{
			name:       "codeSamples with force generate",
			targetType: "go",
			outdir:     "go",
			inputDoc:   "spec.yaml",
			withForce:  true,
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
			workflowFile.Targets["first-target"] = workflow.Target{
				Target: tt.targetType,
				Source: "first-source",
				Output: &tt.outdir,
				CodeSamples: &workflow.CodeSamples{
					Output: "codeSamples.yaml",
				},
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

			codeSamplesPath := filepath.Join(tt.outdir, "codeSamples.yaml")
			content, err := os.ReadFile(codeSamplesPath)
			assert.NoError(t, err, "No readable file %s exists", codeSamplesPath)

			if !strings.Contains(string(content), "update") {
				t.Errorf("Update actions do not exist in the codeSamples file")
			}

			checkForExpectedFiles(t, tt.outdir, expectedFilesByLanguage(tt.targetType))
		})
	}
}
