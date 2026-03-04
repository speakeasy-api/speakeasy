package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_case_insensitive_name_matching_merges_same_type_schemes_across_casings(t *testing.T) {
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
  - BearerAuth: []
paths:
  /owners:
    get:
      operationId: listOwners
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
        - BearerAuth: []
  /owners:
    get:
      operationId: listOwners
      security:
        - BearerAuth: []
      responses:
        "204":
          description: No content
  /vets:
    get:
      operationId: listVets
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
info:
  title: ""
  version: ""
`, string(got))
}
