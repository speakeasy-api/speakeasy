package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_case_insensitive_grouping_with_mixed_types_merges_each_type_independently(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - BearerAuth: []
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT`),
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
      description: OAuth2 Service
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:data: Read data`),
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
      type: oauth2
      description: OAuth2 Service B
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            write:data: Write data`),
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /items:
    get:
      operationId: listItems
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
	namespaces := []string{"svcA", "svcB", "svcC", "svcD"}

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
        - BearerAuth: []
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
        - bearerAuth: []
      responses:
        "204":
          description: No content
  /items:
    get:
      operationId: listItems
      security:
        - BearerAuth: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
    bearerAuth:
      type: oauth2
      description: |-
        OAuth2 Service
        OAuth2 Service B
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:data: Read data
            write:data: Write data
info:
  title: ""
  version: ""
`, string(got))
}
