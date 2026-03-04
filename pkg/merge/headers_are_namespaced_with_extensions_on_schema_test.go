package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_headers_are_namespaced_with_extensions_on_schema(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  headers:
    X-Rate-Limit:
      schema:
        type: integer`),
	}
	namespaces := []string{"api"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  headers:
    api_X-Rate-Limit:
      schema:
        type: integer
        x-speakeasy-name-override: X-Rate-Limit
        x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`, string(got))
}
