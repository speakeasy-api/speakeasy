package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_webhooks_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
webhooks:
  test:
    get:
      responses:
        200:
          description: OK
  test2:
    get:
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
webhooks:
  test2:
    x-test: test
    get:
      x-test: test
      responses:
        x-test: test
        200:
          x-test: test
          description: OK
  test3:
    get:
      responses:
        200:
          description: OK`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
webhooks:
  test:
    get:
      responses:
        200:
          description: OK
  test2:
    get:
      responses:
        200:
          description: OK
          x-test: test
        x-test: test
      x-test: test
    x-test: test
  test3:
    get:
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`, string(got))
}
