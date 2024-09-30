package suggest_test

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/suggest"
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

			overlay, err := suggest.BuildErrorCodesOverlay(ctx, model.Model)
			require.NoError(t, err)

			root := model.Index.GetRootNode()
			err = overlay.ApplyTo(root)
			require.NoError(t, err)

			yaml.NewEncoder(os.Stdout).Encode(root)

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
