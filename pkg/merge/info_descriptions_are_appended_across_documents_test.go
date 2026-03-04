package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_info_descriptions_are_appended_across_documents(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: test
  description: First API description
  summary: First summary`),
		[]byte(`openapi: 3.1
info:
  title: test2
  description: Second API description
  summary: Second summary`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: test2
  description: |-
    First API description
    Second API description
  summary: |-
    First summary
    Second summary
  version: ""
`, string(got))
}
