package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func Test_merge_determinism(t *testing.T) {
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
	got1, err := merge(absSchemas)
	got2, err := merge(absSchemas)
	doc1, err := libopenapi.NewDocumentWithConfiguration(got1, &datamodel.DocumentConfiguration{
		AllowFileReferences:                 true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
	})
	require.NoError(t, err)
	doc2, err := libopenapi.NewDocumentWithConfiguration(got2, &datamodel.DocumentConfiguration{
		AllowFileReferences:                 true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
	})
	require.NoError(t, err)
	documentChanges, errs := libopenapi.CompareDocuments(doc1, doc2)
	require.Len(t, errs, 0)
	// When no changes, CompareDocuments returns nil
	require.Nil(t, documentChanges)
	require.Equal(t, string(got1), string(got2))
}

func Test_merge_Success(t *testing.T) {
	type args struct {
		inSchemas [][]byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "largest doc version wins",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.0.1`),
					[]byte(`openapi: 3.0.0`),
				},
			},
			want: "openapi: 3.0.1\n",
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
			want: `x-foo: bar
openapi: "3.1"
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
                "200":
                    description: OK
            servers:
                - url: http://localhost:8080
                  description: local server
                  x-test: test
    /test2:
        get:
            responses:
                "200":
                    description: OK
            servers:
                - url: https://api.example.com
                  description: production api server
    /test3:
        get:
            servers:
                - url: https://api2.example.com
                - url: https://api.example.com
                  description: production api server
            responses:
                "200":
                    description: OK
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
    - bearerAuth: []
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
tags:
    - name: test
      description: test tag
      x-test: test
`,
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
    - name: test 2
      description: test tag 2 modified
    - name: test 3
      description: test tag 3
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
paths:
    x-test: test
    /test:
        x-test: test
        get:
            x-test: test
            responses:
                x-test: test
                "200":
                    x-test: test
                    description: OK
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
                "200":
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
                "200":
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
    /test4:
        get:
            responses:
                "201":
                    description: Created
    /test1:
        get:
            responses:
                "200":
                    description: OK
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
components:
    x-test: test
    schemas:
        test:
            x-test: test
            type: object
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
    x-test: test
    schemas:
        test:
            type: object
        test2:
            type: object
        test3:
            x-test: test
            type: object
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
            x-test: test
            content:
                application/json:
                    schema:
                        type: object
    headers:
        test:
            description: test
            schema:
                type: string
        test2:
            x-test: test
            description: test
            schema:
                type: string
    securitySchemes:
        test:
            type: http
            scheme: bearer
        test2:
            x-test: test
            type: http
            scheme: bearer
    callbacks:
        test:
            test:
                get:
                    responses:
                        "200":
                            description: OK
        test2:
            x-test: test
            test:
                get:
                    x-test: test
                    responses:
                        "200":
                            description: OK
    examples:
        test2:
            x-test: test
            summary: test
    links:
        test2:
            x-test: test
            description: test
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
webhooks:
    test:
        x-test: test
        get:
            x-test: test
            responses:
                x-test: test
                "200":
                    x-test: test
                    description: OK
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
                "200":
                    description: OK
    test2:
        x-test: test
        get:
            x-test: test
            responses:
                x-test: test
                "200":
                    x-test: test
                    description: OK
    test3:
        get:
            responses:
                "200":
                    description: OK
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
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := merge(tt.args.inSchemas)

			assert.Equal(t, tt.want, string(got))
		})
	}
}
