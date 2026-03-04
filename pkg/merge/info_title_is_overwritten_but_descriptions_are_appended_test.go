package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_info_title_is_overwritten_but_descriptions_are_appended(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: test`),
		[]byte(`openapi: 3.1
info:
  title: test2`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: test2
  version: ""
`, string(got))
}
