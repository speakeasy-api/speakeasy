package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_multiple_schemas_per_namespace_are_all_prefixed(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
    Owner:
      type: object
      properties:
        pet:
          $ref: '#/components/schemas/Pet'`),
	}
	namespaces := []string{"v1"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    v1_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: v1
    v1_Owner:
      type: object
      properties:
        pet:
          $ref: '#/components/schemas/v1_Pet'
      x-speakeasy-name-override: Owner
      x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`, string(got))
}
