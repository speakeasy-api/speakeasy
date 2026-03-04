package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_doc_extensions_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
x-foo: bar
x-bar: baz`),
		[]byte(`openapi: 3.1
x-foo: bar2
x-qux: quux`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
x-foo: bar2
x-bar: baz
info:
  title: ""
  version: ""
x-qux: quux
`, string(got))
}
