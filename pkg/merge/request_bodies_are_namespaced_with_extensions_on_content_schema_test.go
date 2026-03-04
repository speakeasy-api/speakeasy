package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_request_bodies_are_namespaced_with_extensions_on_content_schema(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  requestBodies:
    CreatePet:
      content:
        application/json:
          schema:
            type: object
            properties:
              name:
                type: string`),
	}
	namespaces := []string{"v1"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  requestBodies:
    v1_CreatePet:
      content:
        application/json:
          schema:
            type: object
            properties:
              name:
                type: string
            x-speakeasy-name-override: CreatePet
            x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`, string(got))
}
