package transform

import (
	"bytes"
	"context"
	"testing"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnip_RemoveByOperationID(t *testing.T) {
	ctx := context.Background()

	// Create a buffer to store the snipped spec
	var buf bytes.Buffer

	// Call Snip to remove operations by operation ID
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"deletePet", "findPetsByStatus"}, false, &buf)
	require.NoError(t, err)

	// Parse the snipped spec using the openapi package
	snippedDoc, validationErrors, err := openapi.Unmarshal(ctx, &buf)
	require.NoError(t, err)
	require.NotNil(t, snippedDoc)
	require.Empty(t, validationErrors)

	// Check that the operations are removed
	require.NotNil(t, snippedDoc.Paths)

	// Check /pet/{petId} path - delete should be removed but get should remain
	petPath, ok := snippedDoc.Paths.Get("/pet/{petId}")
	require.True(t, ok, "Path /pet/{petId} should exist")
	require.NotNil(t, petPath.Object)

	assert.Nil(t, petPath.Object.GetOperation(openapi.HTTPMethodDelete), "DELETE operation should be removed")
	assert.NotNil(t, petPath.Object.GetOperation(openapi.HTTPMethodGet), "GET operation should still exist")

	// Check /pet/findByStatus path - should be completely removed
	_, ok = snippedDoc.Paths.Get("/pet/findByStatus")
	assert.False(t, ok, "Path /pet/findByStatus should be completely removed")
}

func TestSnip_RemoveByPathMethod(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Remove operations by path:method format
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"/pet/{petId}:DELETE", "/pet/findByStatus:GET"}, false, &buf)
	require.NoError(t, err)

	snippedDoc, _, err := openapi.Unmarshal(ctx, &buf)
	require.NoError(t, err)
	require.NotNil(t, snippedDoc)

	// Verify operations are removed
	petPath, ok := snippedDoc.Paths.Get("/pet/{petId}")
	require.True(t, ok)
	require.NotNil(t, petPath.Object)

	assert.Nil(t, petPath.Object.GetOperation(openapi.HTTPMethodDelete), "DELETE operation should be removed")
	assert.NotNil(t, petPath.Object.GetOperation(openapi.HTTPMethodGet), "GET operation should still exist")
}

func TestSnip_KeepMode(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Keep only specified operations, remove everything else
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"getPetById"}, true, &buf)
	require.NoError(t, err)

	snippedDoc, _, err := openapi.Unmarshal(ctx, &buf)
	require.NoError(t, err)
	require.NotNil(t, snippedDoc)

	// In keep mode, only getPetById should remain
	petPath, ok := snippedDoc.Paths.Get("/pet/{petId}")
	require.True(t, ok, "Path /pet/{petId} should exist")
	require.NotNil(t, petPath.Object)

	assert.NotNil(t, petPath.Object.GetOperation(openapi.HTTPMethodGet), "GET operation should be kept")
	assert.Nil(t, petPath.Object.GetOperation(openapi.HTTPMethodDelete), "DELETE operation should be removed")

	// Other paths should be removed
	_, ok = snippedDoc.Paths.Get("/pet/findByStatus")
	assert.False(t, ok, "Path /pet/findByStatus should be removed")
}

func TestSnip_MixedOperationFormats(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Mix operation ID and path:method formats
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"deletePet", "/pet/findByStatus:GET"}, false, &buf)
	require.NoError(t, err)

	snippedDoc, _, err := openapi.Unmarshal(ctx, &buf)
	require.NoError(t, err)
	require.NotNil(t, snippedDoc)

	// Both specified operations should be removed
	petPath, ok := snippedDoc.Paths.Get("/pet/{petId}")
	require.True(t, ok)
	assert.Nil(t, petPath.Object.GetOperation(openapi.HTTPMethodDelete))

	_, ok = snippedDoc.Paths.Get("/pet/findByStatus")
	assert.False(t, ok)
}

func TestSnip_NoOperationsSpecified(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Should error when no operations are specified
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{}, false, &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operations specified")
}

func TestSnip_InvalidOperationFormat(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Should error on operation without method (missing colon)
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"/pet"}, false, &buf)
	require.NoError(t, err, "Operation without colon is treated as operation ID, not an error")

	// Should error on empty path or method
	buf.Reset()
	err = Snip(ctx, "../../integration/resources/part1.yaml", []string{":GET"}, false, &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operation format")

	buf.Reset()
	err = Snip(ctx, "../../integration/resources/part1.yaml", []string{"/pet:"}, false, &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operation format")
}

func TestSnip_NonExistentOperation(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer

	// Should not error when removing non-existent operations (graceful handling)
	err := Snip(ctx, "../../integration/resources/part1.yaml", []string{"nonExistentOperation"}, false, &buf)
	require.NoError(t, err)

	// Document should still be valid
	snippedDoc, _, err := openapi.Unmarshal(ctx, &buf)
	require.NoError(t, err)
	require.NotNil(t, snippedDoc)
}
