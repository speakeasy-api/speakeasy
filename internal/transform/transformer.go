package transform

import (
	"context"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"io"
)

type transformer[Args interface{}] struct {
	schemaPath  string
	transformFn func(doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], args Args) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error)
	w           io.Writer
	args        Args
}

func (t transformer[Args]) Do(ctx context.Context) error {
	_, schemaContents, _ := schema.GetSchemaContents(ctx, t.schemaPath, "", "")
	doc, err := libopenapi.NewDocumentWithConfiguration(schemaContents, getConfig())
	if err != nil {
		return errors.NewValidationError("failed to load document", -1, err)
	}
	v3Model, _ := doc.BuildV3Model()

	_, v3Model, err = t.transformFn(doc, v3Model, t.args)
	if err != nil {
		return err
	}
	// render the document to our shard
	bytes, err := v3Model.Model.Render()
	if err != nil {
		return errors.NewValidationError("failed to render document", -1, err)
	}

	_, err = t.w.Write(bytes)
	return err
}

func getConfig() *datamodel.DocumentConfiguration {
	return &datamodel.DocumentConfiguration{
		AllowRemoteReferences:               true,
		IgnorePolymorphicCircularReferences: true,
		IgnoreArrayCircularReferences:       true,
		ExtractRefsSequentially:             true,
	}
}
