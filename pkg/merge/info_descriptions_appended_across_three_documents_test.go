package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_info_descriptions_appended_across_three_documents(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: first
  description: First`),
		[]byte(`openapi: 3.1
info:
  title: second
  description: Second`),
		[]byte(`openapi: 3.1
info:
  title: third
  description: Third`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: third
  description: |-
    First
    Second
    Third
  version: ""
`, string(got))
}
