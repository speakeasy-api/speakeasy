package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_conflicting_security_schemes_from_different_namespaces_coexist(t *testing.T) {
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
  - bearerAuth: []
paths:
  /owners:
    get:
      operationId: listOwners
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
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
        - svcA_bearerAuth: []
  /owners:
    get:
      operationId: listOwners
      security:
        - svcB_bearerAuth: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    svcA_bearerAuth:
      type: http
      scheme: bearer
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: svcA
    svcB_bearerAuth:
      type: apiKey
      name: X-API-Key
      in: header
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: svcB
info:
  title: ""
  version: ""
`, string(got))
}
