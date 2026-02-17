package transform

import (
	"bytes"
	"context"
	"testing"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterOperations(t *testing.T) {
	t.Parallel()

	// Create a buffer to store the filtered spec
	var buf bytes.Buffer

	// Call FilterOperations to remove the delete operation
	err := FilterOperations(context.Background(), "../../integration/resources/part1.yaml", []string{"deletePet", "findPetsByStatus"}, false, true, &buf)
	require.NoError(t, err)

	// Parse the filtered spec
	doc, _, err := openapi.Unmarshal(context.Background(), &buf, openapi.WithSkipValidation())
	require.NoError(t, err)

	// Check that the delete operation is removed
	paths := doc.Paths
	petPathRef, ok := paths.Get("/pet/{petId}")
	require.True(t, ok)
	petPath := petPathRef.GetObject()
	require.NotNil(t, petPath)
	assert.Nil(t, petPath.Delete(), "Delete operation should be removed")

	// Check that the findPetsByStatus operation is removed
	// The entire path should be removed because findPetsByStatus was the only operation in it
	_, ok = paths.Get("/pet/findByStatus")
	require.False(t, ok)

	// Check that other operations still exist
	assert.NotNil(t, petPath.Get(), "Get operation should still exist")

	// Check components
	components := doc.Components

	// Check schemas
	pet, ok := components.Schemas.Get("Pet")
	assert.True(t, ok, "Used schema 'Pet' should still exist")
	assert.NotNil(t, pet)

	// Check for removed schemas
	order, ok := components.Schemas.Get("Order")
	assert.False(t, ok, "Schema 'Order' should be removed")
	assert.Nil(t, order)

	user, ok := components.Schemas.Get("User")
	assert.False(t, ok, "Schema 'User' should be removed")
	assert.Nil(t, user)

	// Check responses
	unauthorized, ok := components.Responses.Get("Unauthorized")
	assert.True(t, ok, "Response 'Unauthorized' should still exist")
	assert.NotNil(t, unauthorized)
}
