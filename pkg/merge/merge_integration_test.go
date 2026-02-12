package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_merge_Integration(t *testing.T) {
	t.Parallel()

	// All three features working together
	got, err := merge(t.Context(), [][]byte{
		[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations v1
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - Pets
      responses:
        200:
          description: List pets v1
  /health:
    get:
      operationId: healthCheck
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations v2
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - pets
      responses:
        200:
          description: List pets v2
  /status:
    get:
      operationId: getStatus
      responses:
        200:
          description: OK`),
	}, []string{"svcA", "svcB"}, true)
	require.NoError(t, err)

	// Tags and operations that differ only in description/summary (and tag casing)
	// are treated as equivalent — last wins, no suffixing or path fragments.
	want := `openapi: "3.1"
tags:
  - name: pets
    description: Pet operations v2
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - pets
      responses:
        200:
          description: List pets v2
  /health:
    get:
      operationId: healthCheck
      responses:
        200:
          description: OK
  /status:
    get:
      operationId: getStatus
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
	assert.Equal(t, want, string(got))
}
