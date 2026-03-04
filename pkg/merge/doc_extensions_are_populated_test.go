package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_doc_extensions_are_populated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1
x-foo: bar`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: ""
  version: ""
x-foo: bar
`, string(got))
}
