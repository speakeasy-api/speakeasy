package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name: "same path method description-only diff treated as equivalent with namespaces",
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
  /pets:
    get:
      responses:
        200:
          description: List pets v2
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
      operationId: listPetsV1
      responses:
        200:
          description: List pets v1`),
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listPetsV2
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
      operationId: listPetsV1
      responses:
        "200":
          description: List pets v1
  /pets#v2:
    get:
      operationId: listPetsV2
      responses:
        "200":
          description: List pets v2
info:
  title: ""
  version: ""
`,
		},
		{
			name: "three docs no namespaces fragment suffix matches document position",
			args: args{
				inSchemas: [][]byte{
					// Doc 1 (counter=1): only /cats, no conflict yet
					[]byte(`openapi: 3.1
paths:
  /cats:
    get:
      operationId: listCats
      responses:
        200:
          description: List cats`),
					// Doc 2 (counter=2): introduces /pets GET
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listPetsV2
      responses:
        200:
          description: List pets v2`),
					// Doc 3 (counter=3): conflicts with doc 2's /pets GET
					[]byte(`openapi: 3.1
paths:
  /pets:
    get:
      operationId: listPetsV3
      responses:
        200:
          description: List pets v3`),
				},
			},
			want: `openapi: "3.1"
paths:
  /cats:
    get:
      operationId: listCats
      responses:
        200:
          description: List cats
  /pets#2:
    get:
      operationId: listPetsV2
      responses:
        "200":
          description: List pets v2
  /pets#3:
    get:
      operationId: listPetsV3
      responses:
        "200":
          description: List pets v3
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
      operationId: listPetsV1
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
      operationId: listPetsV2
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
      operationId: listPetsV1
      responses:
        "200":
          description: List pets v1
  /pets#svcB:
    get:
      operationId: listPetsV2
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
			name: "same operationId same content not falsely duplicated",
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
  /pets:
    get:
      operationId: listPets
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
info:
  title: ""
  version: ""
`,
		},
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
