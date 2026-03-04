package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_info_description_appended_when_only_first_doc_has_description(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
info:
  title: test
  description: Only description`),
		[]byte(`openapi: 3.1
info:
  title: test2`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: test2
  description: Only description
  version: ""
`, string(got))
}
