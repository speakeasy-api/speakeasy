package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_info_description_appended_when_only_second_doc_has_description(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: test`),
		[]byte(`openapi: 3.1
info:
  title: test2
  description: Only description`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: test2
  version: ""
  description: Only description
`, string(got))
}
