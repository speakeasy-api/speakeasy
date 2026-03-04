package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_duplicate_descriptions_with_whitespace_differences_are_deduplicated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: test
  description: "  Same description  "`),
		[]byte(`openapi: 3.1
info:
  title: test2
  description: Same description`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: test2
  description: "Same description"
  version: ""
`, string(got))
}
