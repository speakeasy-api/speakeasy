package transform

import (
	"context"
	"io"
	"os"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/yml"
)

type transformer[Args interface{}] struct {
	r           io.Reader
	schemaPath  string
	jsonOut     bool
	transformFn func(ctx context.Context, schemaPath string, doc *openapi.OpenAPI, args Args) (*openapi.OpenAPI, error)
	w           io.Writer
	args        Args
}

func (t transformer[Args]) Do(ctx context.Context) error {
	if t.r == nil {
		var err error
		t.r, err = os.Open(t.schemaPath)
		if err != nil {
			return err
		}
	}

	doc, _, err := openapi.Unmarshal(ctx, t.r, openapi.WithSkipValidation())
	if err != nil {
		return err
	}

	doc, err = t.transformFn(ctx, t.schemaPath, doc, t.args)
	if err != nil {
		return err
	}

	if err := openapi.Sync(ctx, doc); err != nil {
		return err
	}

	cfg := doc.GetCore().GetConfig()
	if t.jsonOut {
		cfg.OutputFormat = yml.OutputFormatJSON
	} else {
		cfg.OutputFormat = yml.OutputFormatYAML
	}
	ctx = yml.ContextWithConfig(ctx, cfg)
	return openapi.Marshal(ctx, doc, t.w)
}
