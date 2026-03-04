package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_no_global_security_with_mixed_inline_security_marks_unsecured_ops(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		// specA: no global security, one op with inline bearer
		[]byte(`openapi: 3.0.3
info:
  title: Service A
  version: 1.0.0
paths:
  /pets:
    get:
      operationId: listPets
      summary: List all pets
      security:
        - BearerAuth: []
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer`),
		// specB: no global security, one op with inline apikey, one op with no security
		[]byte(`openapi: 3.0.3
info:
  title: Service B
  version: 1.0.0
paths:
  /owners:
    get:
      operationId: listOwners
      summary: List all owners
      security:
        - ApiKeyAuth: []
      responses:
        "204":
          description: No content
  /vets:
    get:
      operationId: listVets
      summary: List all vets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      name: X-API-Key
      in: header`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: 3.0.3
info:
  title: Service B
  version: 1.0.0
paths:
  /pets:
    get:
      operationId: listPets
      summary: List all pets
      security:
        - BearerAuth: []
      responses:
        "204":
          description: No content
  /owners:
    get:
      operationId: listOwners
      summary: List all owners
      security:
        - ApiKeyAuth: []
      responses:
        "204":
          description: No content
  /vets:
    get:
      operationId: listVets
      summary: List all vets
      security:
        - {}
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
    ApiKeyAuth:
      type: apiKey
      name: X-API-Key
      in: header
`, string(got))
}
