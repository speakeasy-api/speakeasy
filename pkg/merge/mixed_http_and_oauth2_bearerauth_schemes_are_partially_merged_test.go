package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_mixed_http_and_oauth2_bearerAuth_schemes_are_partially_merged(t *testing.T) {
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
      type: oauth2
      description: Service A OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets`),
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
      type: oauth2
      description: Service B OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:users: Read users`),
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /vets:
    get:
      operationId: listVets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT`),
	}
	namespaces := []string{"svcA", "svcB", "svcC"}

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
        - bearerAuth: []
      responses:
        "204":
          description: No content
  /vets:
    get:
      operationId: listVets
      security:
        - svcC_bearerAuth: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: |-
        Service A OAuth2
        Service B OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            read:users: Read users
    svcC_bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: svcC
info:
  title: ""
  version: ""
`, string(got))
}
