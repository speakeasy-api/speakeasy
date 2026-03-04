package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_oauth2_security_schemes_with_different_scopes_but_same_tokenUrl_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: Service A OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            write:pets: Write pets`),
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: Service B OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:users: Read users
            write:users: Write users`),
	}
	namespaces := []string{"svcA", "svcB"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: |-
        Service A OAuth2
        Service B OAuth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            write:pets: Write pets
            read:users: Read users
            write:users: Write users
info:
  title: ""
  version: ""
`, string(got))
}
