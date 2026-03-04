package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_four_specs_with_different_servers_do_not_accumulate_servers_across_operations(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
servers:
  - url: https://api.example.com/service-a
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
servers:
  - url: https://api.example.com/service-b
paths:
  /gadgets:
    get:
      operationId: listGadgets
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
servers:
  - url: https://api.example.com/service-c
paths:
  /gizmos:
    get:
      operationId: listGizmos
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
servers:
  - url: https://api.example.com/service-d
paths:
  /thingamajigs:
    get:
      operationId: listThingamajigs
      responses:
        200:
          description: OK`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        200:
          description: OK
      servers:
        - url: https://api.example.com/service-a
  /gadgets:
    get:
      operationId: listGadgets
      servers:
        - url: https://api.example.com/service-b
      responses:
        "200":
          description: OK
  /gizmos:
    get:
      operationId: listGizmos
      servers:
        - url: https://api.example.com/service-c
      responses:
        "200":
          description: OK
  /thingamajigs:
    get:
      operationId: listThingamajigs
      servers:
        - url: https://api.example.com/service-d
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`, string(got))
}
