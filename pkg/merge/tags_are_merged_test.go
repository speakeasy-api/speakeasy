package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_tags_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
tags:
  - name: test
    description: test tag
  - name: test 2
    description: test tag 2`),
		[]byte(`openapi: 3.1
tags:
  - name: test 2
    description: test tag 2 modified
  - name: test 3
    description: test tag 3`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
tags:
  - name: test
    description: test tag
  - name: test 2
    description: test tag 2 modified
  - name: test 3
    description: test tag 3
info:
  title: ""
  version: ""
`, string(got))
}
