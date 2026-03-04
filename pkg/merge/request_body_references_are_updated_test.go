package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_request_body_references_are_updated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  /pets:
    post:
      requestBody:
        $ref: '#/components/requestBodies/CreatePet'
      responses:
        200:
          description: Success
components:
  requestBodies:
    CreatePet:
      content:
        application/json:
          schema:
            type: object`),
	}
	namespaces := []string{"api"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
paths:
  /pets:
    post:
      requestBody:
        $ref: '#/components/requestBodies/api_CreatePet'
      responses:
        200:
          description: Success
components:
  requestBodies:
    api_CreatePet:
      content:
        application/json:
          schema:
            type: object
            x-speakeasy-name-override: CreatePet
            x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`, string(got))
}
