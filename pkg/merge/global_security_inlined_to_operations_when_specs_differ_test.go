package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_global_security_inlined_to_operations_when_specs_differ(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		// specA: global bearer auth, one operation inheriting it
		[]byte(`openapi: 3.0.3
info:
  title: Service A
  version: 1.0.0
security:
  - BearerAuth: []
paths:
  /pets:
    get:
      operationId: listPets
      summary: List all pets
      responses:
        "204":
          description: No content
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer`),
		// specB: no global security, op1 has inline basic, op2 has no security (implicit)
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
        - BasicAuth: []
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
    BasicAuth:
      type: http
      scheme: basic`),
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
      responses:
        "204":
          description: No content
      security:
        - BearerAuth: []
  /owners:
    get:
      operationId: listOwners
      summary: List all owners
      security:
        - BasicAuth: []
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
    BasicAuth:
      type: http
      scheme: basic
`, string(got))
}
