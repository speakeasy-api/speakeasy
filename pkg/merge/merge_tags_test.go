package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name: "different casing same content updates operation tag refs",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations
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
    description: Pet operations
paths:
  /dogs:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK`),
				},
			},
			want: `openapi: "3.1"
tags:
  - name: pets
    description: Pet operations
paths:
  /pets:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK
  /dogs:
    get:
      tags:
        - pets
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`,
		},
		{
			name: "different casing description-only diff is treated as equivalent",
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
  - name: pets
    description: Pet CRUD
info:
  title: ""
  version: ""
`,
		},
		{
			name: "different casing description-only diff is treated as equivalent with namespaces",
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
  - name: pets
    description: Pet CRUD
info:
  title: ""
  version: ""
`,
		},
		{
			name: "three docs same tag description-only diff last wins",
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
  - name: AUTH
    description: Auth v3
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
    description: All pet operations
    x-custom: v1`),
					[]byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet CRUD
    x-custom: v2`),
				},
				namespaces: []string{"svcA", "svcB"},
			},
			want: `openapi: "3.1"
tags:
  - name: Pets_svcA
    description: All pet operations
    x-custom: v1
  - name: pets_svcB
    description: Pet CRUD
    x-custom: v2
info:
  title: ""
  version: ""
`,
		},
		{
			name: "operation tag references updated on rename when content differs",
			args: args{
				inSchemas: [][]byte{
					[]byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations v1
    x-group: alpha
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
    x-group: beta
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
    x-group: alpha
  - name: pets_svcB
    description: Pet operations v2
    x-group: beta
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

	t.Run("without namespaces operation-only tags normalized to first occurrence", func(t *testing.T) {
		t.Parallel()
		got, err := merge(t.Context(), [][]byte{specLower, specUpper}, nil, true)
		require.NoError(t, err)
		result := string(got)

		// Neither spec defines a top-level tags array, so there are no
		// document-level tags to merge. The post-merge normalizeOperationTags
		// pass normalizes operation-level tags using first-occurrence-wins:
		// "health" from specLower is seen first and becomes canonical,
		// so specUpper's "Health" is normalized to "health".
		assert.Contains(t, result, "operationId: getHealthLower")
		assert.Contains(t, result, "operationId: getHealthUpper")
		assert.NotContains(t, result, "tags:\n        - Health")
		assert.Contains(t, result, "tags:\n        - health")
	})

	t.Run("with explicit tag objects description-only diff treated as equivalent", func(t *testing.T) {
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

		// Tags that differ only by description are now treated as equivalent
		// (last one wins), so no suffixing should occur
		assert.Contains(t, result, "name: Health")
		assert.Contains(t, result, "Upper-case health endpoints")

		// Both operations should exist and reference the winning tag name
		assert.Contains(t, result, "operationId: getHealthLower")
		assert.Contains(t, result, "operationId: getHealthUpper")
	})

	t.Run("with explicit tag objects truly different content gets suffixed", func(t *testing.T) {
		t.Parallel()

		specLowerWithTag := []byte(`openapi: 3.1.0
info: { title: Lower, version: 1.0.0 }
tags:
  - name: health
    description: Lower-case health endpoints
    x-group: infra
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
    x-group: monitoring
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

		// Tags differ in extensions, so they should be suffixed
		assert.Contains(t, result, "name: health_lower")
		assert.Contains(t, result, "name: Health_upper")

		// Operation tag references should be updated to suffixed names
		assert.Contains(t, result, "health_lower")
		assert.Contains(t, result, "Health_upper")
	})
}

func Test_merge_OperationTagNormalization(t *testing.T) {
	t.Parallel()

	t.Run("operation tags normalized to document-level tag casing", func(t *testing.T) {
		t.Parallel()

		// Doc1 defines tag "Pets" but operations reference it with mixed casing
		doc1 := []byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations
paths:
  /pets:
    get:
      tags:
        - Pets
      responses:
        200:
          description: OK
  /cats:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK`)

		doc2 := []byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations
paths:
  /dogs:
    get:
      tags:
        - PETS
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{doc1, doc2}, nil, true)
		require.NoError(t, err)

		// Document-level tag should be "pets" (last wins)
		// All operation tags should be normalized to "pets"
		want := `openapi: "3.1"
tags:
  - name: pets
    description: Pet operations
paths:
  /pets:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK
  /cats:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK
  /dogs:
    get:
      tags:
        - pets
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
		assert.Equal(t, want, string(got))
	})

	t.Run("undeclared operation tags with mixed casing normalized to first occurrence", func(t *testing.T) {
		t.Parallel()

		doc1 := []byte(`openapi: 3.1
tags:
  - name: Pets
paths:
  /pets:
    get:
      tags:
        - Pets
        - internal
      responses:
        200:
          description: OK`)

		doc2 := []byte(`openapi: 3.1
tags:
  - name: pets
paths:
  /dogs:
    get:
      tags:
        - pets
        - Internal
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{doc1, doc2}, nil, true)
		require.NoError(t, err)

		// "Pets"/"pets" → "pets" (document-level, last wins)
		// "internal"/"Internal" → "internal" (no document-level def, first occurrence wins)
		want := `openapi: "3.1"
tags:
  - name: pets
paths:
  /pets:
    get:
      tags:
        - pets
        - internal
      responses:
        200:
          description: OK
  /dogs:
    get:
      tags:
        - pets
        - internal
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
		assert.Equal(t, want, string(got))
	})

	t.Run("three docs operation tags all normalized to last winner", func(t *testing.T) {
		t.Parallel()

		got, err := merge(t.Context(), [][]byte{
			[]byte(`openapi: 3.1
tags:
  - name: Auth
    description: Auth v1
paths:
  /login:
    post:
      tags:
        - Auth
      responses:
        200:
          description: OK`),
			[]byte(`openapi: 3.1
tags:
  - name: auth
    description: Auth v2
paths:
  /logout:
    post:
      tags:
        - auth
      responses:
        200:
          description: OK`),
			[]byte(`openapi: 3.1
tags:
  - name: AUTH
    description: Auth v3
paths:
  /refresh:
    post:
      tags:
        - AUTH
      responses:
        200:
          description: OK`),
		}, []string{"v1", "v2", "v3"}, true)
		require.NoError(t, err)

		want := `openapi: "3.1"
tags:
  - name: AUTH
    description: Auth v3
paths:
  /login:
    post:
      tags:
        - AUTH
      responses:
        200:
          description: OK
  /logout:
    post:
      tags:
        - AUTH
      responses:
        "200":
          description: OK
  /refresh:
    post:
      tags:
        - AUTH
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
		assert.Equal(t, want, string(got))
	})

	t.Run("suffixed tags with mismatched operation casing", func(t *testing.T) {
		t.Parallel()

		// Doc1 defines tag "Pets" but its operation references "PETS"
		// Doc2 defines tag "pets" but its operation references "pEtS"
		// Tags differ in content (x-custom) so they get suffixed
		// Operation tags should follow their document's suffixed tag name
		doc1 := []byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations
    x-custom: v1
paths:
  /pets:
    get:
      tags:
        - PETS
      responses:
        200:
          description: OK`)

		doc2 := []byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations
    x-custom: v2
paths:
  /dogs:
    get:
      tags:
        - pEtS
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{doc1, doc2}, []string{"svcA", "svcB"}, true)
		require.NoError(t, err)

		// Tags suffixed: "Pets_svcA" and "pets_svcB"
		// Operation tags should match their document's suffixed tag
		want := `openapi: "3.1"
tags:
  - name: Pets_svcA
    description: Pet operations
    x-custom: v1
  - name: pets_svcB
    description: Pet operations
    x-custom: v2
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
`
		assert.Equal(t, want, string(got))
	})

	t.Run("last wins with mismatched operation casing", func(t *testing.T) {
		t.Parallel()

		// Doc1 defines tag "Pets" but its operation references "PETS"
		// Doc2 defines tag "pets" and its operation also references "pets"
		// Tags are content-equal → last wins ("pets")
		// All operations should be normalized to "pets"
		doc1 := []byte(`openapi: 3.1
tags:
  - name: Pets
    description: Pet operations
paths:
  /pets:
    get:
      tags:
        - PETS
      responses:
        200:
          description: OK`)

		doc2 := []byte(`openapi: 3.1
tags:
  - name: pets
    description: Pet operations
paths:
  /dogs:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{doc1, doc2}, nil, true)
		require.NoError(t, err)

		want := `openapi: "3.1"
tags:
  - name: pets
    description: Pet operations
paths:
  /pets:
    get:
      tags:
        - pets
      responses:
        200:
          description: OK
  /dogs:
    get:
      tags:
        - pets
      responses:
        "200":
          description: OK
info:
  title: ""
  version: ""
`
		assert.Equal(t, want, string(got))
	})
}
