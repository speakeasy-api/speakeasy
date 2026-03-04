package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_externalDocs_are_overwritten(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
externalDocs:
  description: test
  url: https://example.com
  x-test: test`),
		[]byte(`openapi: 3.1
externalDocs:
  description: test2
  url: https://example.com`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
externalDocs:
  description: test2
  url: https://example.com
  x-test: test
info:
  title: ""
  version: ""
`, string(got))
}
