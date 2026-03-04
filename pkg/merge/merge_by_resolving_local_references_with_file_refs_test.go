package merge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
