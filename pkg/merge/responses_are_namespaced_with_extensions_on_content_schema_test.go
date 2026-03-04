package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_responses_are_namespaced_with_extensions_on_content_schema(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  responses:
    NotFound:
      description: Not found
      content:
        application/json:
          schema:
            type: object
            properties:
              message:
                type: string`),
	}
	namespaces := []string{"v1"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  responses:
    v1_NotFound:
      description: Not found
      content:
        application/json:
          schema:
            type: object
            properties:
              message:
                type: string
            x-speakeasy-name-override: NotFound
            x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`, string(got))
}
