package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_oauth2_security_schemes_with_different_tokenUrls_coexist(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - oauth2: []
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth-a.example.com/token
          scopes:
            read:pets: Read pets`),
		[]byte(`openapi: 3.1
security:
  - oauth2: []
paths:
  /owners:
    get:
      operationId: listOwners
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth-b.example.com/token
          scopes:
            read:users: Read users`),
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
        - svcA_oauth2: []
  /owners:
    get:
      operationId: listOwners
      security:
        - svcB_oauth2: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    svcA_oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth-a.example.com/token
          scopes:
            read:pets: Read pets
      x-speakeasy-name-override: oauth2
      x-speakeasy-model-namespace: svcA
    svcB_oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth-b.example.com/token
          scopes:
            read:users: Read users
      x-speakeasy-name-override: oauth2
      x-speakeasy-model-namespace: svcB
info:
  title: ""
  version: ""
`, string(got))
}
