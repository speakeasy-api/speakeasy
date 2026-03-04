package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_unique_security_schemes_across_documents_are_not_namespaced(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
		[]byte(`openapi: 3.1
security:
  - apiKey: []
paths:
  /owners:
    get:
      operationId: listOwners
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    apiKey:
      type: apiKey
      name: X-API-Key
      in: header`),
	}
	namespaces := []string{"svcA", "svcB"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content
      security:
        - bearerAuth: []
  /owners:
    get:
      operationId: listOwners
      security:
        - apiKey: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
    apiKey:
      type: apiKey
      name: X-API-Key
      in: header
info:
  title: ""
  version: ""
`, string(got))
}
