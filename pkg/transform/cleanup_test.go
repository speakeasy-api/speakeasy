package transform

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanupDocument_RemovesEmptyPathsAndTrimsMultiline(t *testing.T) {
	t.Parallel()

	input := `openapi: 3.0.3
info:
  title: Cleanup Test
  version: 1.0.0
paths:
  /empty:
  /pets:
    get:
      description: "First line  \nSecond line  "
      responses:
        '200':
          description: ok
`

	var out bytes.Buffer
	err := CleanupFromReader(context.Background(), bytes.NewBufferString(input), "spec.yaml", &out, true)
	require.NoError(t, err)

	got := out.String()

	// Empty path should be removed
	require.NotContains(t, got, "/empty:")

	// Multiline string should be literal block with trimmed trailing spaces
	require.Contains(t, got, "description: |-\n        First line\n        Second line\n")

	// Indentation should remain 2 spaces for top-level keys
	require.Contains(t, got, "openapi: 3.0.3\ninfo:\n  title: Cleanup Test\n  version: 1.0.0\n")
}
