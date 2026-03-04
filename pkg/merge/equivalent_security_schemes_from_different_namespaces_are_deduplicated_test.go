package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_equivalent_security_schemes_from_different_namespaces_are_deduplicated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: OAuth 2.0 Bearer token from Identity Broker.
      scheme: bearer
      bearerFormat: OAuth2 Access Token`),
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: Bearer token for service accounts.
      scheme: bearer
      bearerFormat: OAuth2 Access Token`),
	}
	namespaces := []string{"svcA", "svcB"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: |-
        OAuth 2.0 Bearer token from Identity Broker.
        Bearer token for service accounts.
      scheme: bearer
      bearerFormat: OAuth2 Access Token
info:
  title: ""
  version: ""
`, string(got))
}
