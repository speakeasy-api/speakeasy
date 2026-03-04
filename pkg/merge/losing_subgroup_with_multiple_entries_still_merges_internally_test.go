package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_losing_subgroup_with_multiple_entries_still_merges_internally(t *testing.T) {
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
      scheme: bearer
      bearerFormat: JWT
      description: HTTP bearer from service A`),
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
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: HTTP bearer from service B`),
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
      description: OAuth2 from service C
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:c: Read C`),
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
      type: oauth2
      description: OAuth2 from service D
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:d: Read D`),
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: OAuth2 from service E
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:e: Read E`),
	}
	namespaces := []string{"svcA", "svcB", "svcC", "svcD", "svcE"}

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
        - svcB_bearerAuth: []
  /owners:
    get:
      operationId: listOwners
      security:
        - svcB_bearerAuth: []
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
        - bearerAuth: []
      responses:
        "204":
          description: No content
  /things:
    get:
      operationId: listThings
      security:
        - bearerAuth: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    bearerAuth:
      type: oauth2
      description: |-
        OAuth2 from service C
        OAuth2 from service D
        OAuth2 from service E
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            read:c: Read C
            read:d: Read D
            read:e: Read E
    svcB_bearerAuth:
      type: http
      description: |-
        HTTP bearer from service A
        HTTP bearer from service B
      scheme: bearer
      bearerFormat: JWT
info:
  title: ""
  version: ""
`, string(got))
}
