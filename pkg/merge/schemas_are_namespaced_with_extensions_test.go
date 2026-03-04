package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_schemas_are_namespaced_with_extensions(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      properties:
        name:
          type: string`),
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer`),
	}
	namespaces := []string{"foo", "bar"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    foo_Pet:
      type: object
      properties:
        name:
          type: string
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: foo
    bar_Pet:
      type: object
      properties:
        id:
          type: integer
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: bar
info:
  title: ""
  version: ""
`, string(got))
}
