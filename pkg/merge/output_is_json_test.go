package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_output_is_json(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1
tags:
  - name: test
    description: test tag
    x-test: test`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, false)
	assert.Equal(t, `{
  "openapi": "3.1",
  "info": {
    "title": "",
    "version": ""
  },
  "tags": [
    {
      "name": "test",
      "description": "test tag",
      "x-test": "test"
    }
  ]
}`, string(got))
}
