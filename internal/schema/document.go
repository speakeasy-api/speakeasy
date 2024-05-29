package schema

import (
	"context"
	"errors"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func LoadDocument(ctx context.Context, path string) ([]byte, *libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	_, schemaContents, _ := GetSchemaContents(ctx, path, "", "")
	doc, err := libopenapi.NewDocumentWithConfiguration(schemaContents, getConfig())
	if err != nil {
		return schemaContents, nil, nil, err
	}
	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		return schemaContents, &doc, v3Model, errors.Join(errs...)
	}
	return schemaContents, &doc, v3Model, err
}

func getConfig() *datamodel.DocumentConfiguration {
	return &datamodel.DocumentConfiguration{
		AllowRemoteReferences:               true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
		ExtractRefsSequentially:             true,
	}
}
