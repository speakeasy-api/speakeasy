package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_WithNamespaces_namespace_count_mismatch_returns_error(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1`),
	}
	namespaces := []string{"foo"}

	_, err := merge(t.Context(), inSchemas, namespaces, true)
	assert.Error(t, err)
}
