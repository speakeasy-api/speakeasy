package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_different_global_security_is_inlined_to_operations_and_cleared_globally(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - apiKey: []
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content`),
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /owners:
    get:
      operationId: listOwners
      responses:
        "204":
          description: No content`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "204":
          description: No content
      security:
        - apiKey: []
  /owners:
    get:
      operationId: listOwners
      security:
        - bearerAuth: []
      responses:
        "204":
          description: No content
info:
  title: ""
  version: ""
`, string(got))
}
