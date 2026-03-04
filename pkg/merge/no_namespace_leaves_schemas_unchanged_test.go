package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_no_namespace_leaves_schemas_unchanged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object`),
	}

	got, err := merge(t.Context(), inSchemas, nil, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    Pet:
      type: object
info:
  title: ""
  version: ""
`, string(got))
}
