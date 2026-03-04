package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_servers_are_moved_to_operations_if_they_dont_share_common_urls(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
servers:
  - url: http://localhost:8080
    description: local server
    x-test: test
paths:
  /test:
    get:
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
servers:
  - url: https://api.example.com
    description: production api server
paths:
  /test2:
    get:
      responses:
        200:
          description: OK
  /test3:
    get:
      servers:
        - url: https://api2.example.com
      responses:
        200:
          description: OK`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
paths:
  /test:
    get:
      responses:
        200:
          description: OK
      servers:
        - url: http://localhost:8080
          description: local server
          x-test: test
  /test2:
    get:
      servers:
        - url: https://api.example.com
          description: production api server
      responses:
        "200":
          description: OK
  /test3:
    get:
      servers:
        - url: https://api2.example.com
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`, string(got))
}
