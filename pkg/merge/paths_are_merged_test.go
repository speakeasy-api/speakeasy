package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_paths_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
paths:
  x-test: test
  /test:
    x-test: test
    get:
      x-test: test
      responses:
        x-test: test
        200:
          x-test: test
          description: OK
  /test3:
    parameters:
      - name: test
        in: query
        schema:
          type: string
      - name: test2
        in: query
        schema:
          type: string
    get:
      responses:
        200:
          description: OK
  /test4:
    get:
      parameters:
       - name: test
         in: query
         schema:
           type: string
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
paths:
  /test1:
    get:
      responses:
        200:
          description: OK
  /test3:
    parameters:
      - name: test2
        in: query
        schema:
          type: object
      - name: test3
        in: query
        schema:
          type: string
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
      responses:
        200:
          description: OK
  /test4:
    get:
      responses:
        201:
          description: Created`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
paths:
  x-test: test
  /test:
    x-test: test
    get:
      x-test: test
      responses:
        x-test: test
        200:
          x-test: test
          description: OK
  /test3:
    parameters:
      - name: test
        in: query
        schema:
          type: string
      - name: test2
        in: query
        schema:
          type: object
      - name: test3
        in: query
        schema:
          type: string
    get:
      responses:
        200:
          description: OK
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
      responses:
        "200":
          description: OK
  /test1:
    get:
      responses:
        "200":
          description: OK
  /test4#1:
    get:
      parameters:
        - name: test
          in: query
          schema:
            type: string
      responses:
        "200":
          description: OK
  /test4#2:
    get:
      responses:
        "201":
          description: Created
info:
  title: ""
  version: ""
`, string(got))
}
