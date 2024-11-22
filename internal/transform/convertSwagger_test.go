package transform

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/require"
)

func TestConvertSwaggerYAML(t *testing.T) {
	// Create a buffer to store the filtered spec
	var buf bytes.Buffer

	// Call ConvertSwagger
	err := ConvertSwagger(context.Background(), "../../integration/resources/swagger.yaml", true, &buf)
	require.NoError(t, err)

	// Parse the converted spec
	filteredDoc, err := libopenapi.NewDocument(buf.Bytes())
	require.NoError(t, err)

	// Validate the OpenAPI v3 model
	model, errors := filteredDoc.BuildV3Model()
	require.Empty(t, errors)

	// Validate that the model exists
	require.NotNil(t, model)

	converted, err := os.ReadFile("../../integration/resources/converted.yaml")
	require.NoError(t, err)

	require.Equal(t, string(converted), buf.String())
}

func TestConvertSwaggerJSON(t *testing.T) {
	// Create a buffer to store the filtered spec
	var buf bytes.Buffer

	// Call ConvertSwagger
	err := ConvertSwagger(context.Background(), "../../integration/resources/swagger.json", false, &buf)
	require.NoError(t, err)

	// Parse the converted spec
	filteredDoc, err := libopenapi.NewDocument(buf.Bytes())
	require.NoError(t, err)

	// Validate the OpenAPI v3 model
	model, errors := filteredDoc.BuildV3Model()
	require.Empty(t, errors)

	// Validate that the model exists
	require.NotNil(t, model)
}
