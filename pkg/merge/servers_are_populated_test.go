package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_servers_are_populated(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1`),
		[]byte(`openapi: 3.1
servers:
  - url: http://localhost:8080
    description: local server`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
info:
  title: ""
  version: ""
servers:
  - url: http://localhost:8080
    description: local server
`, string(got))
}
