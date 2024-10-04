package transform

import (
	"bytes"
	"context"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterOperations(t *testing.T) {
	// Create a buffer to store the filtered spec
	var buf bytes.Buffer

	// Call FilterOperations to remove the delete operation
	err := FilterOperations(context.Background(), "../../integration/resources/part1.yaml", []string{"deletePet"}, false, true, &buf)
	require.NoError(t, err)

	// Parse the filtered spec
	filteredDoc, err := libopenapi.NewDocument(buf.Bytes())
	require.NoError(t, err)

	model, errors := filteredDoc.BuildV3Model()
	require.Empty(t, errors)

	// Check that the delete operation is removed
	paths := model.Model.Paths
	petPath, ok := paths.PathItems.Get("/pet/{petId}")
	require.True(t, ok)
	assert.Nil(t, petPath.Delete, "Delete operation should be removed")

	// Check that other operations still exist
	assert.NotNil(t, petPath.Get, "Get operation should still exist")

	// Check components
	components := model.Model.Components

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

	// Optional: Print the filtered spec for manual inspection
	// fmt.Println(buf.String())
}
