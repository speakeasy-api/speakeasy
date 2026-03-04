package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_components_are_populated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1
components:
  x-test: test
  schemas:
    test:
      x-test: test
      type: object`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: ""
  version: ""
components:
  schemas:
    test:
      type: object
      x-test: test
  x-test: test
`, string(got))
}
