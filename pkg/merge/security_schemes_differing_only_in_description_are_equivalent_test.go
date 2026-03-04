package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_security_schemes_differing_only_in_description_are_equivalent(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: OAuth 2.0 Bearer token from Identity Broker.
      scheme: bearer
      bearerFormat: OAuth2 Access Token`),
		[]byte(`openapi: 3.1
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: Bearer token authentication for service accounts.
      scheme: bearer
      bearerFormat: OAuth2 Access Token`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
components:
  securitySchemes:
    bearerAuth:
      type: http
      description: Bearer token authentication for service accounts.
      scheme: bearer
      bearerFormat: OAuth2 Access Token
info:
  title: ""
  version: ""
`, string(got))
}
