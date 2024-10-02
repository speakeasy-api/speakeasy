package errorCodes_test

import (
	"context"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/suggest/errorCodes"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestBuildErrorCodesOverlay(t *testing.T) {
	type args struct {
		name, in, out string
	}
	toTest := []args{
		{"Reuse 4XX code", "testData/reuse4xx.yaml", "testData/reuse4xx_expected.yaml"},
		{"Simple case -- add all missing", "testData/simple.yaml", "testData/simple_expected.yaml"},
		{"Name conflict in added schema", "testData/nameConflict.yaml", "testData/nameConflict_expected.yaml"},
	}

	for _, tt := range toTest {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, _, model, err := schemas.LoadDocument(ctx, tt.in)
			require.NoError(t, err)

			overlay := errorCodes.BuildErrorCodesOverlay(ctx, model.Model)

			root := model.Index.GetRootNode()
			err = overlay.ApplyTo(root)
			require.NoError(t, err)

			// Read the expected YAML file
			expectedBytes, err := os.ReadFile(tt.out)
			require.NoError(t, err)

			// Convert root to YAML
			actualBytes, err := yaml.Marshal(root)
			require.NoError(t, err)

			// Compare the actual and expected YAML
			require.YAMLEq(t, string(expectedBytes), string(actualBytes))
		})
	}
}

func TestDiagnose(t *testing.T) {
	type args struct {
		name, schema  string
		expectedCount int
	}
	toTest := []args{
		{"Most errors missing", "testData/simple.yaml", 3},
		{"Already defined error codes", "testData/simple_expected.yaml", 0},
	}

	for _, tt := range toTest {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, _, model, err := schemas.LoadDocument(ctx, tt.schema)
			require.NoError(t, err)

			diagnosis := errorCodes.Diagnose(model.Model)
			if tt.expectedCount == 0 {
				require.Len(t, diagnosis, 0)
				return
			}
			require.Len(t, diagnosis, 1)

			diagnostics, ok := diagnosis[suggestions.MissingErrorCodes]
			require.True(t, ok)

			require.Len(t, diagnostics, tt.expectedCount)
		})
	}
}
