package suggest_test

import (
	"context"
	"os"
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
	"github.com/speakeasy-api/speakeasy/internal/suggest/errorCodes"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestDiagnose verifies the top-level Diagnose function returns diagnostics
// This exercises schemas.LoadDocument → openapi.GetOASSummary → suggestions.Diagnose
func TestDiagnose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		schemaPath        string
		expectedDiagTypes int // number of distinct diagnostic types returned
	}
	tests := []testCase{
		{"Simple spec", "errorCodes/testData/simple.yaml", 1},
		{"Simple spec with error codes", "errorCodes/testData/simple_expected.yaml", 1},
		{"Petstore spec", "errorCodes/testData/petstore.yaml", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			diagnosis, err := suggest.Diagnose(ctx, tt.schemaPath)
			require.NoError(t, err)
			require.NotNil(t, diagnosis)
			require.Len(t, diagnosis, tt.expectedDiagTypes)
		})
	}
}

// TestLoadAndApplyOverlay exercises the full path that SuggestAndWrite uses:
// schemas.LoadDocument → build overlay → apply overlay to YAML root → render
// This is the critical path that changes during the migration.
func TestLoadAndApplyOverlay(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name, in, expected string
	}
	tests := []testCase{
		{"Simple case", "errorCodes/testData/simple.yaml", "errorCodes/testData/simple_expected.yaml"},
		{"Petstore", "errorCodes/testData/petstore.yaml", "errorCodes/testData/petstore_expected.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			// 1. Load document via schemas.LoadDocument (the function we're migrating)
			schemaBytes, _, err := schemas.LoadDocument(ctx, tt.in)
			require.NoError(t, err)

			// 2. Build overlay
			overlay, err := errorCodes.BuildErrorCodesOverlay(ctx, tt.in)
			require.NoError(t, err)

			// 3. Get root node from schema bytes (mimics SuggestAndWrite's model.Index.GetRootNode() path)
			var root yaml.Node
			require.NoError(t, yaml.Unmarshal(schemaBytes, &root))

			// 4. Apply overlay
			require.NoError(t, overlay.ApplyTo(&root))

			// 5. Render as YAML
			actualBytes, err := schemas.Render(&root, tt.in, true)
			require.NoError(t, err)

			// 6. Compare with expected
			expectedBytes, err := os.ReadFile(tt.expected)
			require.NoError(t, err)
			require.YAMLEq(t, string(expectedBytes), string(actualBytes))
		})
	}
}
