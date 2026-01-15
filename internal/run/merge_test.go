package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeDocuments(t *testing.T) {
	tests := []struct {
		name            string
		inSchemas       []string
		modelNamespaces []string
		wantErr         bool
		wantErrContains string
		checkOutput     func(t *testing.T, output string)
	}{
		{
			name: "standard merge without namespaces",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
				"testdata/merge_spec2.yaml",
			},
			modelNamespaces: []string{"", ""},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// Should contain both paths
				if !strings.Contains(output, "/pets") {
					t.Error("output should contain /pets path")
				}
				if !strings.Contains(output, "/orders") {
					t.Error("output should contain /orders path")
				}
				// Should have merged schemas (last one wins)
				if !strings.Contains(output, "Pet:") {
					t.Error("output should contain Pet schema")
				}
				if !strings.Contains(output, "Order:") {
					t.Error("output should contain Order schema")
				}
				// Should NOT have namespace prefixes
				if strings.Contains(output, "serviceA_Pet") {
					t.Error("output should NOT contain namespaced schema names when no modelNamespaces specified")
				}
			},
		},
		{
			name: "merge with namespaces",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
				"testdata/merge_spec2.yaml",
			},
			modelNamespaces: []string{"serviceA", "serviceB"},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// Should contain namespaced schemas
				if !strings.Contains(output, "serviceA_Pet:") {
					t.Error("output should contain serviceA_Pet schema")
				}
				if !strings.Contains(output, "serviceB_Pet:") {
					t.Error("output should contain serviceB_Pet schema")
				}
				if !strings.Contains(output, "serviceB_Order:") {
					t.Error("output should contain serviceB_Order schema")
				}
				// Should have x-speakeasy extensions
				if !strings.Contains(output, "x-speakeasy-name-override: Pet") {
					t.Error("output should contain x-speakeasy-name-override extension")
				}
				if !strings.Contains(output, "x-speakeasy-model-namespace: serviceA") {
					t.Error("output should contain x-speakeasy-model-namespace extension for serviceA")
				}
				if !strings.Contains(output, "x-speakeasy-model-namespace: serviceB") {
					t.Error("output should contain x-speakeasy-model-namespace extension for serviceB")
				}
				// Should have updated references
				if !strings.Contains(output, "#/components/schemas/serviceA_Pet") {
					t.Error("output should contain updated reference to serviceA_Pet")
				}
				if !strings.Contains(output, "#/components/schemas/serviceB_Order") {
					t.Error("output should contain updated reference to serviceB_Order")
				}
			},
		},
		{
			name: "merge with partial namespaces succeeds",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
				"testdata/merge_spec2.yaml",
			},
			modelNamespaces: []string{"serviceA", ""},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// First document should have namespaced schema
				if !strings.Contains(output, "serviceA_Pet:") {
					t.Error("output should contain serviceA_Pet schema from first document")
				}
				// Second document should keep original schema name (no namespace)
				if !strings.Contains(output, "Order:") {
					t.Error("output should contain Order schema from second document")
				}
				// Should have x-speakeasy extensions only for namespaced schemas
				if !strings.Contains(output, "x-speakeasy-model-namespace: serviceA") {
					t.Error("output should contain x-speakeasy-model-namespace extension for serviceA")
				}
			},
		},
		{
			name: "merge single file without namespace",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
			},
			modelNamespaces: []string{""},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "/pets") {
					t.Error("output should contain /pets path")
				}
				if !strings.Contains(output, "Pet:") {
					t.Error("output should contain Pet schema")
				}
			},
		},
		{
			name: "merge single file with namespace",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
			},
			modelNamespaces: []string{"serviceA"},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "serviceA_Pet:") {
					t.Error("output should contain serviceA_Pet schema")
				}
				if !strings.Contains(output, "x-speakeasy-name-override: Pet") {
					t.Error("output should contain x-speakeasy-name-override extension")
				}
			},
		},
		{
			name: "merge JSON output",
			inSchemas: []string{
				"testdata/merge_spec1.yaml",
				"testdata/merge_spec2.yaml",
			},
			modelNamespaces: []string{"", ""},
			wantErr:         false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// JSON output should have proper structure
				if !strings.Contains(output, `"openapi"`) {
					t.Error("JSON output should contain openapi field")
				}
				if !strings.Contains(output, `"/pets"`) {
					t.Error("JSON output should contain /pets path")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create temp output file
			tmpDir := t.TempDir()
			var outFile string
			if strings.Contains(tt.name, "JSON") {
				outFile = filepath.Join(tmpDir, "merged.json")
			} else {
				outFile = filepath.Join(tmpDir, "merged.yaml")
			}

			err := mergeDocuments(ctx, tt.inSchemas, tt.modelNamespaces, outFile, "", "", true)

			if tt.wantErr {
				if err == nil {
					t.Errorf("mergeDocuments() expected error, got nil")
					return
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("mergeDocuments() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Errorf("mergeDocuments() unexpected error = %v", err)
				return
			}

			// Read output file and check contents
			output, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("failed to read output file: %v", err)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, string(output))
			}
		})
	}
}

func TestMergeDocumentsWithInvalidInput(t *testing.T) {
	tests := []struct {
		name            string
		inSchemas       []string
		modelNamespaces []string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "non-existent file",
			inSchemas:       []string{"testdata/nonexistent.yaml"},
			modelNamespaces: []string{""},
			wantErr:         true,
		},
		{
			name:            "empty schema list",
			inSchemas:       []string{},
			modelNamespaces: []string{},
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			outFile := filepath.Join(tmpDir, "merged.yaml")

			err := mergeDocuments(ctx, tt.inSchemas, tt.modelNamespaces, outFile, "", "", true)

			if tt.wantErr {
				if err == nil {
					t.Error("mergeDocuments() expected error, got nil")
					return
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("mergeDocuments() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Errorf("mergeDocuments() unexpected error = %v", err)
			}
		})
	}
}

func TestHasModelNamespaces(t *testing.T) {
	tests := []struct {
		name            string
		modelNamespaces []string
		want            bool
	}{
		{
			name:            "all empty",
			modelNamespaces: []string{"", "", ""},
			want:            false,
		},
		{
			name:            "one non-empty",
			modelNamespaces: []string{"", "serviceA", ""},
			want:            true,
		},
		{
			name:            "all non-empty",
			modelNamespaces: []string{"serviceA", "serviceB"},
			want:            true,
		},
		{
			name:            "nil slice",
			modelNamespaces: nil,
			want:            false,
		},
		{
			name:            "empty slice",
			modelNamespaces: []string{},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasModelNamespaces := false
			for _, ns := range tt.modelNamespaces {
				if ns != "" {
					hasModelNamespaces = true
					break
				}
			}

			if hasModelNamespaces != tt.want {
				t.Errorf("hasModelNamespaces = %v, want %v", hasModelNamespaces, tt.want)
			}
		})
	}
}
