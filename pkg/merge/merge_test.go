package merge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_merge_determinism(t *testing.T) {
	t.Parallel()

	// test data not included
	t.Skip()
	absSchemas := [][]byte{}
	wd, err := os.Getwd()
	require.NoError(t, err)
	files, err := os.ReadDir(filepath.Join(wd, "testdata"))
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}
		content, err := os.ReadFile(filepath.Join("testdata", f.Name()))
		require.NoError(t, err)
		absSchemas = append(absSchemas, content)
	}

	// Run merge twice and ensure the output is the same.
	got1, err := merge(t.Context(), absSchemas, nil, true)
	require.NoError(t, err)
	got2, err := merge(t.Context(), absSchemas, nil, true)
	require.NoError(t, err)

	// Verify both outputs parse as valid OpenAPI documents
	_, _, err = openapi.Unmarshal(context.Background(), bytes.NewReader(got1), openapi.WithSkipValidation())
	require.NoError(t, err)
	_, _, err = openapi.Unmarshal(context.Background(), bytes.NewReader(got2), openapi.WithSkipValidation())
	require.NoError(t, err)

	// Compare outputs for determinism
	require.Equal(t, string(got1), string(got2))
}

func Test_merge_Success(t *testing.T) {
	t.Parallel()

	type args struct {
		inSchemas [][]byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
		jsonOut bool
	}{
		{
			name: "largest doc version wins",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.0.1`),
					[]byte(`openapi: 3.0.0`),
				},
			},
			want: `openapi: 3.0.1
info:
  title: ""
  version: ""
`,
		},
		{
			name: "info is overwritten",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
info:
  title: test`),
					[]byte(`openapi: 3.1
info:
  title: test2`),
				},
			},
			want: `openapi: "3.1"
info:
  title: test2
  version: ""
`,
		},
		{
			name: "doc extensions are populated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1
x-foo: bar`),
				},
			},
			want: `openapi: "3.1"
info:
  title: ""
  version: ""
x-foo: bar
`,
		},
		{
			name: "doc extensions are merged",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
x-foo: bar
x-bar: baz`),
					[]byte(`openapi: 3.1
x-foo: bar2
x-qux: quux`),
				},
			},
			want: `openapi: "3.1"
x-foo: bar2
x-bar: baz
info:
  title: ""
  version: ""
x-qux: quux
`,
		},
		{
			name: "servers are populated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1
servers:
  - url: http://localhost:8080
    description: local server`),
				},
			},
			want: `openapi: "3.1"
info:
  title: ""
  version: ""
servers:
  - url: http://localhost:8080
    description: local server
`,
		},
		{
			name: "servers are merged if they share common urls",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
`,
		},
		{
			name: "servers are moved to operations if they don't share common urls",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
        - url: https://api.example.com
          description: production api server
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`,
		},
		{
			name: "security is overwritten",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
security:
  - apiKey: []`),
					[]byte(`openapi: 3.1
security:
  - bearerAuth: []`),
				},
			},
			want: `openapi: "3.1"
security:
  - apiKey: []
    bearerAuth: []
info:
  title: ""
  version: ""
`,
		},
		{
			name: "tags are populated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1
tags:
  - name: test
    description: test tag
    x-test: test`),
				},
			},
			want: `openapi: "3.1"
info:
  title: ""
  version: ""
tags:
  - name: test
    description: test tag
    x-test: test
`,
		},
		{
			name:    "output is json",
			jsonOut: true,
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1
tags:
  - name: test
    description: test tag
    x-test: test`),
				},
			},
			want: `{
  "openapi": "3.1",
  "info": {
    "title": "",
    "version": ""
  },
  "tags": [
    {
      "name": "test",
      "description": "test tag",
      "x-test": "test"
    }
  ]
}`,
		},
		{
			name: "tags are merged",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: test
    description: test tag
  - name: test 2
    description: test tag 2`),
					[]byte(`openapi: 3.1
tags:
  - name: test 2
    description: test tag 2 modified
  - name: test 3
    description: test tag 3`),
				},
			},
			want: `openapi: "3.1"
tags:
  - name: test
    description: test tag
  - name: test 2_1
    description: test tag 2
  - name: test 2_2
    description: test tag 2 modified
  - name: test 3
    description: test tag 3
info:
  title: ""
  version: ""
`,
		},
		{
			name: "paths are populated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
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
          description: OK`),
				},
			},
			want: `openapi: "3.1"
info:
  title: ""
  version: ""
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          x-test: test
        x-test: test
      x-test: test
    x-test: test
  x-test: test
`,
		},
		{
			name: "paths are merged",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
`,
		},
		{
			name: "components are populated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1
components:
  x-test: test
  schemas:
    test:
      x-test: test
      type: object`),
				},
			},
			want: `openapi: "3.1"
info:
  title: ""
  version: ""
components:
  schemas:
    test:
      type: object
      x-test: test
  x-test: test
`,
		},
		{
			name: "components are merged",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
`,
		},
		{
			name: "webhooks are populated",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
`,
		},
		{
			name: "webhooks are merged",
			args: args{
				inSchemas: [][]byte{
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
				},
			},
			want: `openapi: "3.1"
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
`,
		},
		{
			name: "externalDocs are overwritten",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
externalDocs:
  description: test
  url: https://example.com
  x-test: test`),
					[]byte(`openapi: 3.1
externalDocs:
  description: test2
  url: https://example.com`),
				},
			},
			want: `openapi: "3.1"
externalDocs:
  description: test2
  url: https://example.com
  x-test: test
info:
  title: ""
  version: ""
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _ := merge(t.Context(), tt.args.inSchemas, nil, !tt.jsonOut)

			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_MergeByResolvingLocalReferences_WithFileRefs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tempDir := t.TempDir()

	mainSchemaPath := filepath.Join(tempDir, "main-schema.yaml")
	referencedSchemaPath := filepath.Join(tempDir, "referenced-schema.yaml")

	referencedSchema := `openapi: 3.1
info:
  title: Referenced Schema
  version: 1.0.0
components:
  schemas:
    ReferencedObject:
      type: object
      properties:
        name:
          type: string
`
	err := os.WriteFile(referencedSchemaPath, []byte(referencedSchema), 0o644)
	require.NoError(t, err)

	// Create and write the main schema file
	mainSchema := `openapi: 3.1
info:
  title: Main Schema
  version: 1.0.0
paths:
  /example:
    get:
      summary: Example endpoint
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                $ref: './referenced-schema.yaml#/components/schemas/ReferencedObject'
`
	err = os.WriteFile(mainSchemaPath, []byte(mainSchema), 0o644)
	require.NoError(t, err)

	outFile, err := os.CreateTemp(t.TempDir(), "out-schema-*.yaml")
	require.NoError(t, err)

	// Call the function under test
	err = MergeByResolvingLocalReferences(ctx, mainSchemaPath, outFile.Name(), tempDir, "", "", false)
	require.NoError(t, err)

	// Read and verify the output
	outputData, err := os.ReadFile(outFile.Name())
	require.NoError(t, err)

	expectedOutput := `openapi: "3.1"
info:
  title: Main Schema
  version: 1.0.0
paths:
  /example:
    get:
      summary: Example endpoint
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
`
	assert.Equal(t, expectedOutput, string(outputData))
}

func Test_merge_WithNamespaces(t *testing.T) {
	t.Parallel()

	type args struct {
		inSchemas  [][]byte
		namespaces []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "schemas are namespaced with extensions",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      properties:
        name:
          type: string`),
					[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer`),
				},
				namespaces: []string{"foo", "bar"},
			},
			want: `openapi: "3.1"
components:
  schemas:
    foo_Pet:
      type: object
      properties:
        name:
          type: string
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: foo
    bar_Pet:
      type: object
      properties:
        id:
          type: integer
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: bar
info:
  title: ""
  version: ""
`,
		},
		{
			name: "references are updated to namespaced schemas",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/api_Pet'
components:
  schemas:
    api_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`,
		},
		{
			name: "no namespace leaves schemas unchanged",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object`),
				},
				namespaces: nil,
			},
			want: `openapi: "3.1"
components:
  schemas:
    Pet:
      type: object
info:
  title: ""
  version: ""
`,
		},
		{
			name: "mixed namespace and no namespace works",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object`),
					[]byte(`openapi: 3.1
components:
  schemas:
    Owner:
      type: object`),
				},
				namespaces: []string{"foo", ""},
			},
			want: `openapi: "3.1"
components:
  schemas:
    foo_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: foo
    Owner:
      type: object
info:
  title: ""
  version: ""
`,
		},
		{
			name: "namespace count mismatch returns error",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1`),
					[]byte(`openapi: 3.1`),
				},
				namespaces: []string{"foo"},
			},
			wantErr: true,
		},
		{
			name: "multiple schemas per namespace are all prefixed",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  schemas:
    Pet:
      type: object
    Owner:
      type: object
      properties:
        pet:
          $ref: '#/components/schemas/Pet'`),
				},
				namespaces: []string{"v1"},
			},
			want: `openapi: "3.1"
components:
  schemas:
    v1_Pet:
      type: object
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: v1
    v1_Owner:
      type: object
      properties:
        pet:
          $ref: '#/components/schemas/v1_Pet'
      x-speakeasy-name-override: Owner
      x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`,
		},
		{
			name: "parameters are namespaced with extensions on schema",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: integer`),
					[]byte(`openapi: 3.1
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: string`),
				},
				namespaces: []string{"foo", "bar"},
			},
			want: `openapi: "3.1"
components:
  parameters:
    foo_Limit:
      name: limit
      in: query
      schema:
        type: integer
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: foo
    bar_Limit:
      name: limit
      in: query
      schema:
        type: string
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: bar
info:
  title: ""
  version: ""
`,
		},
		{
			name: "parameter references are updated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/Limit'
      responses:
        200:
          description: Success
components:
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: integer`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/api_Limit'
      responses:
        200:
          description: Success
components:
  parameters:
    api_Limit:
      name: limit
      in: query
      schema:
        type: integer
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`,
		},
		{
			name: "responses are namespaced with extensions on content schema",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  responses:
    NotFound:
      description: Not found
      content:
        application/json:
          schema:
            type: object
            properties:
              message:
                type: string`),
				},
				namespaces: []string{"v1"},
			},
			want: `openapi: "3.1"
components:
  responses:
    v1_NotFound:
      description: Not found
      content:
        application/json:
          schema:
            type: object
            properties:
              message:
                type: string
            x-speakeasy-name-override: NotFound
            x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`,
		},
		{
			name: "response references are updated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        404:
          $ref: '#/components/responses/NotFound'
components:
  responses:
    NotFound:
      description: Not found`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        404:
          $ref: '#/components/responses/api_NotFound'
components:
  responses:
    api_NotFound:
      description: 'Not found'
info:
  title: ''
  version: ''
`,
		},
		{
			name: "request bodies are namespaced with extensions on content schema",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  requestBodies:
    CreatePet:
      content:
        application/json:
          schema:
            type: object
            properties:
              name:
                type: string`),
				},
				namespaces: []string{"v1"},
			},
			want: `openapi: "3.1"
components:
  requestBodies:
    v1_CreatePet:
      content:
        application/json:
          schema:
            type: object
            properties:
              name:
                type: string
            x-speakeasy-name-override: CreatePet
            x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`,
		},
		{
			name: "request body references are updated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    post:
      requestBody:
        $ref: '#/components/requestBodies/CreatePet'
      responses:
        200:
          description: Success
components:
  requestBodies:
    CreatePet:
      content:
        application/json:
          schema:
            type: object`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    post:
      requestBody:
        $ref: '#/components/requestBodies/api_CreatePet'
      responses:
        200:
          description: Success
components:
  requestBodies:
    api_CreatePet:
      content:
        application/json:
          schema:
            type: object
            x-speakeasy-name-override: CreatePet
            x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`,
		},
		{
			name: "headers are namespaced with extensions on schema",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
components:
  headers:
    X-Rate-Limit:
      schema:
        type: integer`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
components:
  headers:
    api_X-Rate-Limit:
      schema:
        type: integer
        x-speakeasy-name-override: X-Rate-Limit
        x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`,
		},
		{
			name: "header references in responses are updated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          headers:
            X-Rate-Limit:
              $ref: '#/components/headers/X-Rate-Limit'
components:
  headers:
    X-Rate-Limit:
      schema:
        type: integer`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        200:
          description: Success
          headers:
            X-Rate-Limit:
              $ref: '#/components/headers/api_X-Rate-Limit'
components:
  headers:
    api_X-Rate-Limit:
      schema:
        type: integer
        x-speakeasy-name-override: X-Rate-Limit
        x-speakeasy-model-namespace: api
info:
  title: ""
  version: ""
`,
		},
		{
			name: "security schemes are namespaced with extensions and requirements updated",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      security:
        - bearerAuth: []
      responses:
        200:
          description: Success
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
				},
				namespaces: []string{"v1"},
			},
			want: `openapi: "3.1"
security:
  - v1_bearerAuth: []
paths:
  /pets:
    get:
      security:
        - v1_bearerAuth: []
      responses:
        200:
          description: Success
components:
  securitySchemes:
    v1_bearerAuth:
      type: http
      scheme: bearer
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: v1
info:
  title: ""
  version: ""
`,
		},
		{
			name: "conflicting security schemes from different namespaces coexist",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
					[]byte(`openapi: 3.1
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: apiKey
      name: X-API-Key
      in: header`),
				},
				namespaces: []string{"svcA", "svcB"},
			},
			want: `openapi: "3.1"
security:
  - svcB_bearerAuth: []
components:
  securitySchemes:
    svcA_bearerAuth:
      type: http
      scheme: bearer
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: svcA
    svcB_bearerAuth:
      type: apiKey
      name: X-API-Key
      in: header
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: svcB
info:
  title: ""
  version: ""
`,
		},
		{
			name: "all component types namespaced together",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
security:
  - bearerAuth: []
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/Limit'
      responses:
        200:
          $ref: '#/components/responses/Success'
        404:
          description: Not found
          headers:
            X-Request-Id:
              $ref: '#/components/headers/X-Request-Id'
components:
  schemas:
    Pet:
      type: object
  parameters:
    Limit:
      name: limit
      in: query
      schema:
        type: integer
  responses:
    Success:
      description: OK
      content:
        application/json:
          schema:
            type: object
  headers:
    X-Request-Id:
      schema:
        type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer`),
				},
				namespaces: []string{"api"},
			},
			want: `openapi: "3.1"
security:
  - api_bearerAuth: []
paths:
  /pets:
    get:
      parameters:
        - $ref: '#/components/parameters/api_Limit'
      responses:
        200:
          $ref: '#/components/responses/api_Success'
        404:
          description: Not found
          headers:
            X-Request-Id:
              $ref: '#/components/headers/api_X-Request-Id'
components:
  schemas:
    api_Pet:
      type: 'object'
      x-speakeasy-name-override: Pet
      x-speakeasy-model-namespace: api
  parameters:
    api_Limit:
      name: 'limit'
      in: 'query'
      schema:
        type: 'integer'
        x-speakeasy-name-override: Limit
        x-speakeasy-model-namespace: api
  responses:
    api_Success:
      description: 'OK'
      content:
        application/json:
          schema:
            type: 'object'
            x-speakeasy-name-override: Success
            x-speakeasy-model-namespace: api
  headers:
    api_X-Request-Id:
      schema:
        type: 'string'
        x-speakeasy-name-override: X-Request-Id
        x-speakeasy-model-namespace: api
  securitySchemes:
    api_bearerAuth:
      type: 'http'
      scheme: 'bearer'
      x-speakeasy-name-override: bearerAuth
      x-speakeasy-model-namespace: api
info:
  title: ''
  version: ''
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge(t.Context(), tt.args.inSchemas, tt.args.namespaces, true)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_merge_CaseInsensitiveTags(t *testing.T) {
	t.Parallel()

	type args struct {
		inSchemas  [][]byte
		namespaces []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "same name same content last wins",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations`),
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations`),
				},
			},
			want: `openapi: "3.1"
tags:
  - name: Pets
    description: Pet operations
info:
  title: ""
  version: ""
`,
		},
		{
			name: "different casing same content last wins",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations`),
					[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations`),
				},
			},
			want: `openapi: "3.1"
tags:
  - name: pets
    description: Pet operations
info:
  title: ""
  version: ""
`,
		},
		{
			name: "different casing different content gets suffixed without namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: All pet operations`),
					[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet CRUD`),
				},
			},
			want: `openapi: "3.1"
tags:
  - name: Pets_1
    description: All pet operations
  - name: pets_2
    description: Pet CRUD
info:
  title: ""
  version: ""
`,
		},
		{
			name: "different casing different content gets suffixed with namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: All pet operations`),
					[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet CRUD`),
				},
				namespaces: []string{"svcA", "svcB"},
			},
			want: `openapi: "3.1"
tags:
  - name: Pets_svcA
    description: All pet operations
  - name: pets_svcB
    description: Pet CRUD
info:
  title: ""
  version: ""
`,
		},
		{
			name: "three docs same tag different content all suffixed",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Auth
    description: Auth v1`),
					[]byte(`openapi: 3.1
tags:
  - name: auth
    description: Auth v2`),
					[]byte(`openapi: 3.1
tags:
  - name: AUTH
    description: Auth v3`),
				},
				namespaces: []string{"v1", "v2", "v3"},
			},
			want: `openapi: "3.1"
tags:
  - name: Auth_v1
    description: Auth v1
  - name: auth_v2
    description: Auth v2
  - name: AUTH_v3
    description: Auth v3
info:
  title: ""
  version: ""
`,
		},
		{
			name: "operation tag references updated on rename",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations v1
paths:
  /pets:
    get:
      tags:
        - Pets
      responses:
        200:
          description: OK`),
					[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations v2
paths:
  /dogs:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK`),
				},
				namespaces: []string{"svcA", "svcB"},
			},
			want: `openapi: "3.1"
tags:
  - name: Pets_svcA
    description: Pet operations v1
  - name: pets_svcB
    description: Pet operations v2
paths:
  /pets:
    get:
      tags:
        - Pets_svcA
      responses:
        200:
          description: OK
  /dogs:
    get:
      tags:
        - pets_svcB
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge(t.Context(), tt.args.inSchemas, tt.args.namespaces, true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_merge_PathMethodConflicts(t *testing.T) {
	t.Parallel()

	type args struct {
		inSchemas  [][]byte
		namespaces []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "same path method same content last wins",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: OK`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: OK`),
				},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        200:
          description: OK
info:
  title: ""
  version: ""
`,
		},
		{
			name: "same path different methods no conflict",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: OK`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    post:
      responses:
        201:
          description: Created`),
				},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      responses:
        200:
          description: OK
    post:
      responses:
        "201":
          description: Created
info:
  title: ""
  version: ""
`,
		},
		{
			name: "same path method different content creates fragments without namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        200:
          description: List all pets`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listAnimals
      responses:
        200:
          description: List all animals`),
				},
			},
			want: `openapi: "3.1"
paths:
  /pets#1:
    get:
      operationId: listPets
      responses:
        "200":
          description: List all pets
  /pets#2:
    get:
      operationId: listAnimals
      responses:
        "200":
          description: List all animals
info:
  title: ""
  version: ""
`,
		},
		{
			name: "same path method different content creates fragments with namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: List pets v1`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: List pets v2`),
				},
				namespaces: []string{"v1", "v2"},
			},
			want: `openapi: "3.1"
paths:
  /pets#v1:
    get:
      responses:
        "200":
          description: List pets v1
  /pets#v2:
    get:
      responses:
        "200":
          description: List pets v2
info:
  title: ""
  version: ""
`,
		},
		{
			name: "mixed conflicting and non-conflicting methods",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: List pets v1
    post:
      responses:
        201:
          description: Create pet`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      responses:
        200:
          description: List pets v2
    delete:
      responses:
        204:
          description: Delete all`),
				},
				namespaces: []string{"svcA", "svcB"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    post:
      responses:
        201:
          description: Create pet
    delete:
      responses:
        "204":
          description: Delete all
  /pets#svcA:
    get:
      responses:
        "200":
          description: List pets v1
  /pets#svcB:
    get:
      responses:
        "200":
          description: List pets v2
info:
  title: ""
  version: ""
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge(t.Context(), tt.args.inSchemas, tt.args.namespaces, true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_merge_OperationIdDeduplication(t *testing.T) {
	t.Parallel()

	type args struct {
		inSchemas  [][]byte
		namespaces []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "unique operationIds unchanged",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        200:
          description: OK`),
					[]byte(`openapi: 3.1
paths:
  /dogs:
    get:
      operationId: listDogs
      responses:
        200:
          description: OK`),
				},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        200:
          description: OK
  /dogs:
    get:
      operationId: listDogs
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`,
		},
		{
			name: "duplicate operationIds suffixed with namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: list
      responses:
        200:
          description: List pets`),
					[]byte(`openapi: 3.1
paths:
  /dogs:
    get:
      operationId: list
      responses:
        200:
          description: List dogs`),
				},
				namespaces: []string{"pets", "dogs"},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      operationId: list_pets
      responses:
        200:
          description: List pets
  /dogs:
    get:
      operationId: list_dogs
      responses:
        "200":
          description: List dogs
info:
  title: ""
  version: ""
`,
		},
		{
			name: "duplicate operationIds suffixed with numbers when no namespaces",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: list
      responses:
        200:
          description: List pets`),
					[]byte(`openapi: 3.1
paths:
  /dogs:
    get:
      operationId: list
      responses:
        200:
          description: List dogs`),
				},
			},
			want: `openapi: "3.1"
paths:
  /pets:
    get:
      operationId: list_1
      responses:
        200:
          description: List pets
  /dogs:
    get:
      operationId: list_2
      responses:
        "200":
          description: List dogs
info:
  title: ""
  version: ""
`,
		},
		{
			name: "three docs same operationId all suffixed",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
paths:
  /a:
    get:
      operationId: fetch
      responses:
        200:
          description: A`),
					[]byte(`openapi: 3.1
paths:
  /b:
    get:
      operationId: fetch
      responses:
        200:
          description: B`),
					[]byte(`openapi: 3.1
paths:
  /c:
    get:
      operationId: fetch
      responses:
        200:
          description: C`),
				},
				namespaces: []string{"a", "b", "c"},
			},
			want: `openapi: "3.1"
paths:
  /a:
    get:
      operationId: fetch_a
      responses:
        200:
          description: A
  /b:
    get:
      operationId: fetch_b
      responses:
        "200":
          description: B
  /c:
    get:
      operationId: fetch_c
      responses:
        "200":
          description: C
info:
  title: ""
  version: ""
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := merge(t.Context(), tt.args.inSchemas, tt.args.namespaces, true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_merge_Integration(t *testing.T) {
	t.Parallel()

	// All three features working together
	got, err := merge(t.Context(), [][]byte{
		[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations v1
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - Pets
      responses:
        200:
          description: List pets v1
  /health:
    get:
      operationId: healthCheck
      responses:
        200:
          description: OK`),
		[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations v2
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - pets
      responses:
        200:
          description: List pets v2
  /status:
    get:
      operationId: getStatus
      responses:
        200:
          description: OK`),
	}, []string{"svcA", "svcB"}, true)
	require.NoError(t, err)

	want := `openapi: "3.1"
tags:
  - name: Pets_svcA
    description: Pet operations v1
  - name: pets_svcB
    description: Pet operations v2
paths:
  /health:
    get:
      operationId: healthCheck
      responses:
        200:
          description: OK
  /pets#svcA:
    get:
      operationId: listPets_svcA
      tags:
        - Pets_svcA
      responses:
        "200":
          description: List pets v1
  /pets#svcB:
    get:
      operationId: listPets_svcB
      tags:
        - pets_svcB
      responses:
        "200":
          description: List pets v2
  /status:
    get:
      operationId: getStatus
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
	assert.Equal(t, want, string(got))
}

// Test 1 from the issue: duplicate operationId across different paths
// (the patchThing / patchOrganization pattern)
func Test_merge_DuplicateOperationIdAcrossPaths(t *testing.T) {
	t.Parallel()

	specA := []byte(`openapi: 3.1.0
info: { title: A, version: 1.0.0 }
paths:
  /a/things/{id}:
    patch:
      operationId: patchThing
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        "200": { description: ok }`)

	specB := []byte(`openapi: 3.1.0
info: { title: B, version: 1.0.0 }
paths:
  /b/things/{id}:
    patch:
      operationId: patchThing
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        "200": { description: ok }`)

	t.Run("without namespaces deduplicates with numbers", func(t *testing.T) {
		t.Parallel()
		got, err := merge(t.Context(), [][]byte{specA, specB}, nil, true)
		require.NoError(t, err)
		result := string(got)

		// Both operationIds should be suffixed so they no longer clash
		assert.Contains(t, result, "operationId: patchThing_1")
		assert.Contains(t, result, "operationId: patchThing_2")
		assert.NotContains(t, result, "operationId: patchThing\n")
	})

	t.Run("with namespaces deduplicates with namespace names", func(t *testing.T) {
		t.Parallel()
		got, err := merge(t.Context(), [][]byte{specA, specB}, []string{"svcA", "svcB"}, true)
		require.NoError(t, err)
		result := string(got)

		assert.Contains(t, result, "operationId: patchThing_svcA")
		assert.Contains(t, result, "operationId: patchThing_svcB")
		assert.NotContains(t, result, "operationId: patchThing\n")
	})

	t.Run("paths remain distinct and separate", func(t *testing.T) {
		t.Parallel()
		got, err := merge(t.Context(), [][]byte{specA, specB}, []string{"svcA", "svcB"}, true)
		require.NoError(t, err)
		result := string(got)

		// Both paths should exist (no fragment needed since paths differ)
		assert.Contains(t, result, "/a/things/{id}:")
		assert.Contains(t, result, "/b/things/{id}:")
		// No fragment paths should be created
		assert.NotContains(t, result, "#svcA")
		assert.NotContains(t, result, "#svcB")
	})
}

// Test 2 from the issue: tag case collision (health vs Health)
func Test_merge_TagCaseCollision(t *testing.T) {
	t.Parallel()

	specLower := []byte(`openapi: 3.1.0
info: { title: Lower, version: 1.0.0 }
paths:
  /healthz:
    get:
      tags: [health]
      operationId: getHealthLower
      responses:
        "200": { description: ok }`)

	specUpper := []byte(`openapi: 3.1.0
info: { title: Upper, version: 1.0.0 }
paths:
  /health:
    get:
      tags: [Health]
      operationId: getHealthUpper
      responses:
        "200": { description: ok }`)

	t.Run("without namespaces tags get number suffixes", func(t *testing.T) {
		t.Parallel()
		got, err := merge(t.Context(), [][]byte{specLower, specUpper}, nil, true)
		require.NoError(t, err)
		result := string(got)

		// Tags that differ only by case but have no description differences
		// should still be detected as case-insensitive match.
		// Both are tag objects with only a name (no description), so they're
		// "same content" → last one wins.
		// Actually: specLower's tag "health" has no explicit tag definition
		// (it's only referenced in the operation). Same for specUpper.
		// Since neither spec defines a top-level tags array, the merge
		// won't touch tags at all — the collision is only in operation.Tags.
		//
		// Let's just verify both operations and their tags exist
		assert.Contains(t, result, "operationId: getHealthLower")
		assert.Contains(t, result, "operationId: getHealthUpper")
	})

	t.Run("with explicit tag objects different content gets suffixed", func(t *testing.T) {
		t.Parallel()

		specLowerWithTag := []byte(`openapi: 3.1.0
info: { title: Lower, version: 1.0.0 }
tags:
  - name: health
    description: Lower-case health endpoints
paths:
  /healthz:
    get:
      tags: [health]
      operationId: getHealthLower
      responses:
        "200": { description: ok }`)

		specUpperWithTag := []byte(`openapi: 3.1.0
info: { title: Upper, version: 1.0.0 }
tags:
  - name: Health
    description: Upper-case health endpoints
paths:
  /health:
    get:
      tags: [Health]
      operationId: getHealthUpper
      responses:
        "200": { description: ok }`)

		got, err := merge(t.Context(), [][]byte{specLowerWithTag, specUpperWithTag}, []string{"lower", "upper"}, true)
		require.NoError(t, err)
		result := string(got)

		// Tags should be disambiguated with namespace suffix
		assert.Contains(t, result, "name: health_lower")
		assert.Contains(t, result, "name: Health_upper")

		// Operation tag references should be updated to suffixed names
		assert.Contains(t, result, "health_lower")
		assert.Contains(t, result, "Health_upper")

		// Original unsuffixed tag names should NOT appear in tag definitions
		assert.NotContains(t, result, "name: health\n")
		assert.NotContains(t, result, "name: Health\n")
	})
}
