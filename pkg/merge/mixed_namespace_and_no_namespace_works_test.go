package merge

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_merge_WithNamespaces_mixed_namespace_and_no_namespace_works(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object`),
		[]byte(`openapi: 3.1
components:
  schemas:
    Owner:
      type: object`),
	}
	namespaces := []string{"foo", ""}

	got, err := merge(t.Context(), inSchemas, namespaces, true)
	require.NoError(t, err)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    foo_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: foo
    Owner:
      type: object
info:
  title: ""
  version: ""
`, string(got))
}
