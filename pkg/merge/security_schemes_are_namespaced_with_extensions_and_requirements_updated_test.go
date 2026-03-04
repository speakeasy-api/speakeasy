package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_security_schemes_are_namespaced_with_extensions_and_requirements_updated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      security:
        - bearerAuth: []
      responses:
        200:
          description: Success
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
	}
	namespaces := []string{"v1"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      security:
        - bearerAuth: []
      responses:
        200:
          description: Success
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
info:
  title: ""
  version: ""
`, string(got))
}
