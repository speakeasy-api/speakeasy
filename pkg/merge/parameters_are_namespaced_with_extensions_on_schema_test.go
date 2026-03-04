package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_parameters_are_namespaced_with_extensions_on_schema(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: integer`),
		[]byte(`openapi: 3.1
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: string`),
	}
	namespaces := []string{"foo", "bar"}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  parameters:
    foo_Limit:
      name: limit
      in: query
      schema:
        type: integer
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: foo
    bar_Limit:
      name: limit
      in: query
      schema:
        type: string
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: bar
info:
  title: ""
  version: ""
`, string(got))
}
