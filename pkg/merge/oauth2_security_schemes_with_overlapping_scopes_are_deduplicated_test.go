package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_oauth2_security_schemes_with_overlapping_scopes_are_deduplicated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            shared:scope: Shared scope`),
		[]byte(`openapi: 3.1
security:
  - oauth2: []
components:
  securitySchemes:
    oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:users: Read users
            shared:scope: Shared scope`),
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
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:pets: Read pets
            shared:scope: Shared scope
            read:users: Read users
info:
  title: ""
  version: ""
`, string(got))
}
