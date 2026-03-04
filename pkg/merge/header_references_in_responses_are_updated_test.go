package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_header_references_in_responses_are_updated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          headers:
            X-Rate-Limit:
              $ref: '#/components/headers/X-Rate-Limit'
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
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          headers:
            X-Rate-Limit:
              $ref: '#/components/headers/api_X-Rate-Limit'
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
