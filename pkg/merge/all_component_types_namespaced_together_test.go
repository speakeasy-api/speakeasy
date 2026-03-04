package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_all_component_types_namespaced_together(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/Limit'
      responses:
        200:
          $ref: '#/components/responses/Success'
        404:
          description: Not found
          headers:
            X-Request-Id:
              $ref: '#/components/headers/X-Request-Id'
components:
  schemas:
    Pet:
      type: object
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: integer
  responses:
    Success:
      description: OK
      content:
        application/json:
          schema:
            type: object
  headers:
    X-Request-Id:
      schema:
        type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
	}
	namespaces := []string{"api"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/api_Limit'
      responses:
        200:
          $ref: '#/components/responses/api_Success'
        404:
          description: Not found
          headers:
            X-Request-Id:
              $ref: '#/components/headers/api_X-Request-Id'
components:
  schemas:
    api_Pet:
      type: 'object'
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: api
  parameters:
    api_Limit:
      name: 'limit'
      in: 'query'
      schema:
        type: 'integer'
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: api
  responses:
    api_Success:
      description: 'OK'
      content:
        application/json:
          schema:
            type: 'object'
            x-speakeasy-name-override: Success
            x-speakeasy-model-namespace: api
  headers:
    api_X-Request-Id:
      schema:
        type: 'string'
        x-speakeasy-name-override: X-Request-Id
        x-speakeasy-model-namespace: api
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
info:
  title: ''
  version: ''
`, string(got))
}
