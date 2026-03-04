package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_largest_doc_version_wins(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.0.1`),
		[]byte(`openapi: 3.0.0`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: 3.0.1
info:
  title: ""
  version: ""
`, string(got))
}
