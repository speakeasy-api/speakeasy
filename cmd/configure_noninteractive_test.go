package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContext creates a context suitable for testing with a logger attached.
func testContext() context.Context {
	return log.With(context.Background(), log.New())
}

func TestConfigureSourcesNonInteractive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flags       ConfigureSourcesFlags
		setup       func() *workflow.Workflow
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, wf *workflow.Workflow)
	}{
		{
			name: "creates new source with URL location",
			flags: ConfigureSourcesFlags{
				Location:   "https://petstore.swagger.io/v2/swagger.json",
				SourceName: "petstore",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: make(map[string]workflow.Source),
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				source, ok := wf.Sources["petstore"]
				assert.True(t, ok, "source 'petstore' should exist")
				assert.Len(t, source.Inputs, 1)
				assert.Equal(t, "https://petstore.swagger.io/v2/swagger.json", source.Inputs[0].Location.Reference())
			},
		},
		{
			name: "creates source with auth header",
			flags: ConfigureSourcesFlags{
				Location:   "https://api.example.com/openapi.yaml",
				SourceName: "my-api",
				AuthHeader: "X-API-Key",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: make(map[string]workflow.Source),
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				source := wf.Sources["my-api"]
				require.NotNil(t, source.Inputs[0].Auth)
				assert.Equal(t, "X-API-Key", source.Inputs[0].Auth.Header)
				assert.Equal(t, "$openapi_doc_auth_token", source.Inputs[0].Auth.Secret)
			},
		},
		{
			name: "creates source with output path",
			flags: ConfigureSourcesFlags{
				Location:   "https://api.example.com/openapi.yaml",
				SourceName: "my-api",
				OutputPath: "compiled/openapi.yaml",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: make(map[string]workflow.Source),
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				source := wf.Sources["my-api"]
				require.NotNil(t, source.Output)
				assert.Equal(t, "compiled/openapi.yaml", *source.Output)
			},
		},
		{
			name: "fails when source name already exists",
			flags: ConfigureSourcesFlags{
				Location:   "https://api.example.com/openapi.yaml",
				SourceName: "existing-source",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"existing-source": {
							Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}},
						},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			expectError: true,
			errorMsg:    "already exists",
		},
		{
			name: "fails when source name contains spaces",
			flags: ConfigureSourcesFlags{
				Location:   "https://api.example.com/openapi.yaml",
				SourceName: "my source",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: make(map[string]workflow.Source),
					Targets: make(map[string]workflow.Target),
				}
			},
			expectError: true,
			errorMsg:    "must not contain spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp directory
			tmpDir := t.TempDir()

			// Create .speakeasy directory
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".speakeasy"), 0o755))

			// Run setup to get workflow
			var wf *workflow.Workflow
			if tt.setup != nil {
				wf = tt.setup()
			}

			// Run non-interactive configuration
			err := configureSourcesNonInteractive(testContext(), tmpDir, wf, tt.flags)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)

			// Validate results on the in-memory workflow
			if tt.validate != nil {
				tt.validate(t, wf)
			}

			// Also verify the workflow was saved to disk
			loadedWf, _, err := workflow.Load(tmpDir)
			require.NoError(t, err)
			require.NotNil(t, loadedWf)
			_, ok := loadedWf.Sources[tt.flags.SourceName]
			assert.True(t, ok, "source should be saved to disk")
		})
	}
}

func TestConfigureTargetNonInteractive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flags       ConfigureTargetFlags
		setup       func() *workflow.Workflow
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, wf *workflow.Workflow)
	}{
		{
			name: "creates typescript target",
			flags: ConfigureTargetFlags{
				TargetType: "typescript",
				SourceID:   "my-source",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"my-source": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				target, ok := wf.Targets["typescript"]
				assert.True(t, ok, "target 'typescript' should exist")
				assert.Equal(t, "typescript", target.Target)
				assert.Equal(t, "my-source", target.Source)
			},
		},
		{
			name: "creates target with custom name",
			flags: ConfigureTargetFlags{
				TargetType: "python",
				SourceID:   "api-source",
				TargetName: "my-python-sdk",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"api-source": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				_, ok := wf.Targets["my-python-sdk"]
				assert.True(t, ok, "target 'my-python-sdk' should exist")
			},
		},
		{
			name: "creates target with output directory",
			flags: ConfigureTargetFlags{
				TargetType: "go",
				SourceID:   "api",
				OutputDir:  "sdk/go",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"api": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			validate: func(t *testing.T, wf *workflow.Workflow) {
				t.Helper()
				target := wf.Targets["go"]
				require.NotNil(t, target.Output)
				assert.Equal(t, "sdk/go", *target.Output)
			},
		},
		{
			name: "fails with unsupported target type",
			flags: ConfigureTargetFlags{
				TargetType: "unsupported-lang",
				SourceID:   "api",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"api": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			expectError: true,
			errorMsg:    "unsupported target type",
		},
		{
			name: "fails when source doesn't exist",
			flags: ConfigureTargetFlags{
				TargetType: "typescript",
				SourceID:   "nonexistent",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"existing-source": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: make(map[string]workflow.Target),
				}
			},
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "fails when target name already exists",
			flags: ConfigureTargetFlags{
				TargetType: "typescript",
				SourceID:   "api",
				TargetName: "existing-target",
			},
			setup: func() *workflow.Workflow {
				return &workflow.Workflow{
					Version: workflow.WorkflowVersion,
					Sources: map[string]workflow.Source{
						"api": {Inputs: []workflow.Document{{Location: "https://example.com/api.yaml"}}},
					},
					Targets: map[string]workflow.Target{
						"existing-target": {Target: "python", Source: "api"},
					},
				}
			},
			expectError: true,
			errorMsg:    "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp directory
			tmpDir := t.TempDir()

			// Create .speakeasy directory
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".speakeasy"), 0o755))

			// Run setup to get workflow
			var wf *workflow.Workflow
			if tt.setup != nil {
				wf = tt.setup()
			}

			// Run non-interactive configuration
			err := configureTargetNonInteractive(testContext(), tmpDir, wf, tt.flags)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)

			// Validate results on the in-memory workflow
			if tt.validate != nil {
				tt.validate(t, wf)
			}

			// Also verify the workflow was saved to disk
			loadedWf, _, err := workflow.Load(tmpDir)
			require.NoError(t, err)
			require.NotNil(t, loadedWf)

			expectedTargetName := tt.flags.TargetName
			if expectedTargetName == "" {
				expectedTargetName = tt.flags.TargetType
			}
			_, ok := loadedWf.Targets[expectedTargetName]
			assert.True(t, ok, "target should be saved to disk")
		})
	}
}
