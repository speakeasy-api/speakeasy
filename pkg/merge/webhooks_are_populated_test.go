package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_webhooks_are_populated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1
webhooks:
  test:
    x-test: test
    get:
      x-test: test
      responses:
        x-test: test
        200:
          x-test: test
          description: OK`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: ""
  version: ""
webhooks:
  test:
    get:
      responses:
        "200":
          description: OK
          x-test: test
        x-test: test
      x-test: test
    x-test: test
`, string(got))
}
