package transform

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJQSymbolicExecutionFromReader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "basic openapi document",
			input: `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    get:
      summary: Test endpoint
      responses:
        '200':
          description: Success
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer

			err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, output.String())
			}
		})
	}
}

func TestJQSymbolicExecution_ExtractNestedID(t *testing.T) {
	input := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    UserResponse:
      type: object
      x-speakeasy-transform-from-api:
        jq: '. + {id: .data.user.id}'
      properties:
        data:
          type: object
          properties:
            user:
              type: object
              properties:
                id:
                  type: string
                name:
                  type: string
`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)
	require.NoError(t, err)

	expected := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    UserResponse:
      type: object
      x-speakeasy-transform-from-api:
        jq: '. + {id: .data.user.id}'
      properties:
        data:
          type: object
          properties:
            user:
              type: object
              properties:
                id:
                  type: string
                name:
                  type: string
        id:
          type: string
      required:
        - id
`

	assert.Equal(t, expected, output.String())
}

func TestJQSymbolicExecution_FlattenPagination(t *testing.T) {
	input := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    ItemsResponse:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{items: .data.items, total: .data.pagination.total, hasMore: (.data.pagination.nextCursor != null)}'
      properties:
        data:
          type: object
          properties:
            items:
              type: array
              items:
                type: object
                properties:
                  id:
                    type: string
                  title:
                    type: string
            pagination:
              type: object
              properties:
                total:
                  type: integer
                nextCursor:
                  type: string
                  nullable: true
`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)
	require.NoError(t, err)

	expected := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    ItemsResponse:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{items: .data.items, total: .data.pagination.total, hasMore: (.data.pagination.nextCursor != null)}'
      properties:
        hasMore:
          type: boolean
        items:
          type: array
          items:
            type: object
            properties:
              id:
                type: string
              title:
                type: string
        total:
          type: integer
      required:
        - hasMore
        - items
        - total
`

	assert.Equal(t, expected, output.String())
}

func TestJQSymbolicExecution_ComputedField(t *testing.T) {
	input := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Product:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{name, total: (.price * .quantity)}'
      properties:
        name:
          type: string
        price:
          type: number
        quantity:
          type: integer
`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)
	require.NoError(t, err)

	expected := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Product:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{name, total: (.price * .quantity)}'
      properties:
        name:
          type: string
        total:
          type: number
      required:
        - name
        - total
`

	assert.Equal(t, expected, output.String())
}

func TestJQSymbolicExecution_ArrayTransform(t *testing.T) {
	input := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    TagList:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{tags: (.tags | map({value: ., slug: (. | ascii_downcase)}))}'
      properties:
        tags:
          type: array
          items:
            type: string
`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)
	require.NoError(t, err)

	expected := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    TagList:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{tags: (.tags | map({value: ., slug: (. | ascii_downcase)}))}'
      properties:
        tags:
          type: array
          items:
            type: object
            properties:
              slug:
                type: string
              value:
                type: string
            required:
              - slug
              - value
      required:
        - tags
`

	assert.Equal(t, expected, output.String())
}

func TestJQSymbolicExecution_ConditionalField(t *testing.T) {
	input := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    User:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{id, name, tier: (if .score >= 90 then "gold" else "silver" end)}'
      properties:
        id:
          type: integer
        name:
          type: string
        score:
          type: integer
`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := JQSymbolicExecutionFromReader(context.Background(), reader, "test.yaml", true, &output)
	require.NoError(t, err)

	expected := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    User:
      type: object
      x-speakeasy-transform-from-api:
        jq: '{id, name, tier: (if .score >= 90 then "gold" else "silver" end)}'
      properties:
        id:
          type: integer
        name:
          type: string
        tier:
          type: string
          enum:
            - silver
            - gold
      required:
        - id
        - name
        - tier
`

	assert.Equal(t, expected, output.String())
}
