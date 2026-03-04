package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_parameter_references_are_updated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/Limit'
      responses:
        200:
          description: Success
components:
  parameters:
    Limit:
      name: limit
      in: query
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
      parameters:
        - $ref: '#/components/parameters/api_Limit'
      responses:
        200:
          description: Success
components:
  parameters:
    api_Limit:
      name: limit
      in: query
      schema:
        type: integer
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`, string(got))
}
