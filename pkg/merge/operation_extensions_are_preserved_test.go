package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_merge_operation_extensions_are_preserved reproduces GEN-2688 where
// operation-level extensions (x-confluence-page-id, x-internal, x-speakeasy-test,
// x-speakeasy-group, x-speakeasy-name-override) were "flapping around" after merge.
// The root cause was non-deterministic insertion order in the openapi library's
// unmarshaller: extension keys were processed in concurrent goroutines and the mutex
// serialised insertions but did not enforce document order.
func Test_merge_operation_extensions_are_preserved(t *testing.T) {
	t.Parallel()

	// Scenario: two different source specs each contain the same operation with the
	// same extensions. Because the operations are identical they should NOT conflict —
	// the result is a single merged path with extensions preserved in document order.
	t.Run("identical_operations_with_extensions_are_not_fragmented", func(t *testing.T) {
		t.Parallel()

		specA := []byte(`openapi: 3.1
paths:
  /users/move:
    post:
      operationId: moveUsers
      x-confluence-page-id: '553587772'
      x-internal: true
      x-speakeasy-test: true
      x-speakeasy-group: config
      x-speakeasy-name-override: moveUsers
      responses:
        200:
          description: OK`)

		specB := []byte(`openapi: 3.1  # different source, same operation
paths:
  /users/move:
    post:
      operationId: moveUsers
      x-confluence-page-id: '553587772'
      x-internal: true
      x-speakeasy-test: true
      x-speakeasy-group: config
      x-speakeasy-name-override: moveUsers
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{specA, specB}, nil, true)
		assert.NoError(t, err)
		// Should produce a single /users/move path, not fragmented, with extensions
		// preserved in document order (x-confluence-page-id before x-internal).
		assert.Equal(t, `openapi: "3.1"
paths:
  /users/move:
    post:
      operationId: moveUsers
      x-confluence-page-id: '553587772'
      x-internal: true
      x-speakeasy-test: true
      x-speakeasy-group: config
      x-speakeasy-name-override: moveUsers
      responses:
        200:
          description: OK
info:
  title: ""
  version: ""
`, string(got))
	})

	// Scenario: same operation with extensions merged with itself — extensions should
	// be preserved and the operation should not be fragmented.
	t.Run("same_operation_with_extensions_merged_with_itself", func(t *testing.T) {
		t.Parallel()

		spec := []byte(`openapi: 3.1
paths:
  /users/move:
    post:
      operationId: moveUsers
      x-confluence-page-id: '553587772'
      x-internal: true
      x-speakeasy-test: true
      x-speakeasy-group: config
      x-speakeasy-name-override: moveUsers
      responses:
        200:
          description: OK`)

		got, err := merge(t.Context(), [][]byte{spec, spec}, nil, true)
		assert.NoError(t, err)
		// Extensions should be preserved and operation should not be fragmented.
		assert.Equal(t, `openapi: "3.1"
paths:
  /users/move:
    post:
      operationId: moveUsers
      x-confluence-page-id: '553587772'
      x-internal: true
      x-speakeasy-test: true
      x-speakeasy-group: config
      x-speakeasy-name-override: moveUsers
      responses:
        200:
          description: OK
info:
  title: ""
  version: ""
`, string(got))
	})
}
