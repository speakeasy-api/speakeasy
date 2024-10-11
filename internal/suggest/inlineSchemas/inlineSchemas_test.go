package inlineSchemas_test

import (
	"context"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/suggest/inlineSchemas"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func TestInlineSchemas(t *testing.T) {
	type args struct {
		name, in, out string
	}
	toTest := []args{
		{"Simple petstore", "testData/petstore.yaml", "testData/petstore_expected.yaml"},
		{"Simple speakeasy", "testData/simple.yaml", "testData/simple_expected.yaml"},
	}

	for _, tt := range toTest {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			overlay, err := inlineSchemas.RefactorInlineSchemas(ctx, tt.in)

			_, _, model, err := schemas.LoadDocument(ctx, tt.in)
			require.NoError(t, err)
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
