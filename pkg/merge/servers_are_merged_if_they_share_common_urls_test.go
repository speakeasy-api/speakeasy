package merge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_merge_servers_are_merged_if_they_share_common_urls(t *testing.T) {
	t.Parallel()

	inSchemas := [][]byte{
		[]byte(`openapi: 3.1
servers:
  - url: http://localhost:8080
    description: local server
    x-test: test
  - url: https://api.example.com
    description: production server`),
		[]byte(`openapi: 3.1
servers:
  - url: http://localhost:8081
    description: local server 2
  - url: https://api.example.com
    description: production api server`),
	}

	got, _ := merge(t.Context(), inSchemas, nil, true)
	assert.Equal(t, `openapi: "3.1"
servers:
  - url: http://localhost:8080
    description: local server
    x-test: test
  - url: https://api.example.com
    description: production api server
  - url: http://localhost:8081
    description: local server 2
info:
  title: ""
  version: ""
`, string(got))
}
