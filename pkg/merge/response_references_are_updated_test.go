package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_response_references_are_updated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        404:
          $ref: '#/components/responses/NotFound'
components:
  responses:
    NotFound:
      description: Not found`),
	}
	namespaces := []string{"api"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        404:
          $ref: '#/components/responses/api_NotFound'
components:
  responses:
    api_NotFound:
      description: 'Not found'
info:
  title: ''
  version: ''
`, string(got))
}
