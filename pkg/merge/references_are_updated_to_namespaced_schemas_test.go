package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_references_are_updated_to_namespaced_schemas(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object`),
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
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/api_Pet'
components:
  schemas:
    api_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`, string(got))
}
