package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_three_oauth2_schemes_from_three_services_are_merged_with_scopes_unioned(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: Service A
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets`),
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: Service B
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:users: Read users`),
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      description: Service C
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:orders: Read orders`),
	}
	namespaces := []string{"svcA", "svcB", "svcC"}

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
        Service A
        Service B
        Service C
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            read:users: Read users
            read:orders: Read orders
info:
  title: ""
  version: ""
`, string(got))
}
