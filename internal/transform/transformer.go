package transform

import (
	"context"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"io"
)

type transformer[Args interface{}] struct {
	schemaPath  string
	transformFn func(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], args Args) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error)
	w           io.Writer
	args        Args
}

func (t transformer[Args]) Do(ctx context.Context) error {
	_, doc, model, err := openapi.LoadDocument(ctx, t.schemaPath)
	if err != nil {
		return err
	}

	_, model, err = t.transformFn(ctx, *doc, model, t.args)
	if err != nil {
		return err
	}
	// render the document to our shard
	bytes, err := model.Model.Render()
	if err != nil {
		return errors.NewValidationError("failed to render document", -1, err)
	}

	_, err = t.w.Write(bytes)
	return err
}
