package transform

import (
	"context"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"io"
)

type transformer[Args interface{}] struct {
	schemaPath  string
	jsonOut     bool
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

	bytes, err := schemas.Render(model.Index.GetRootNode(), t.schemaPath, !t.jsonOut)
	if err != nil {
		return err
	}

	_, err = t.w.Write(bytes)
	return err
}
