package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_components_are_merged(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
components:
  schemas:
    test:
      type: object
    test2:
      type: string
  responses:
    test:
      description: test
  parameters:
    test:
      name: test
      in: query
      type: string
  requestBodies:
    test:
      content:
        application/json:
          schema:
            type: object
  headers:
    test:
      description: test
      schema:
        type: string
  securitySchemes:
    test:
      type: http
      scheme: bearer
  callbacks:
    test:
      test:
        get:
          responses:
            200:
              description: OK`),
		[]byte(`openapi: 3.1
components:
  x-test: test
  schemas:
    test3:
      x-test: test
      type: object
    test2:
      type: object
  responses:
    test2:
      description: test
      x-test: test
  parameters:
    test2:
      name: test
      in: query
      type: string
      x-test: test
  examples:
    test2:
      x-test: test
      summary: test
  requestBodies:
    test2:
      x-test: test
      content:
        application/json:
          schema:
            type: object
  headers:
    test2:
      x-test: test
      description: test
      schema:
        type: string
  securitySchemes:
    test2:
      x-test: test
      type: http
      scheme: bearer
  links:
    test2:
      x-test: test
      description: test
  callbacks:
    test2:
      x-test: test
      test:
        get:
          x-test: test
          responses:
            200:
              description: OK`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
components:
  schemas:
    test:
      type: object
    test2:
      type: object
    test3:
      type: object
      x-test: test
  responses:
    test:
      description: test
    test2:
      description: test
      x-test: test
  parameters:
    test:
      name: test
      in: query
      type: string
    test2:
      name: test
      in: query
      x-test: test
  requestBodies:
    test:
      content:
        application/json:
          schema:
            type: object
    test2:
      content:
        application/json:
          schema:
            type: object
      x-test: test
  headers:
    test:
      description: test
      schema:
        type: string
    test2:
      description: test
      schema:
        type: string
      x-test: test
  securitySchemes:
    test:
      type: http
      scheme: bearer
    test2:
      type: http
      scheme: bearer
      x-test: test
  callbacks:
    test:
      test:
        get:
          responses:
            200:
              description: OK
    test2:
      test:
        get:
          responses:
            "200":
              description: OK
          x-test: test
      x-test: test
  examples:
    test2:
      summary: test
      x-test: test
  links:
    test2:
      description: test
      x-test: test
  x-test: test
info:
  title: ""
  version: ""
`, string(got))
}
