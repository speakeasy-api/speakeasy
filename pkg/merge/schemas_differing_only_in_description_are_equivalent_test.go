package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_schemas_differing_only_in_description_are_equivalent(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      description: A pet in the store
      properties:
        name:
          type: string`),
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      description: A pet object
      properties:
        name:
          type: string`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    Pet:
      type: object
      description: A pet object
      properties:
        name:
          type: string
info:
  title: ""
  version: ""
`, string(got))
}
