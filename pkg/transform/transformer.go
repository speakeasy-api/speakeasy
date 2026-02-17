package transform

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/speakeasy-api/openapi/openapi"
	"github.com/speakeasy-api/openapi/yml"
	"gopkg.in/yaml.v3"
)

type transformer[Args interface{}] struct {
	r           io.Reader
	schemaPath  string
	jsonOut     bool
	transformFn func(ctx context.Context, doc *openapi.OpenAPI, args Args) (*openapi.OpenAPI, error)
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

	doc, err = t.transformFn(ctx, doc, t.args)
	if err != nil {
		return err
	}

	// Configure output format
	if core := doc.GetCore(); core != nil {
		config := core.Config
		if config == nil {
			config = yml.GetDefaultConfig()
		}
		if t.jsonOut {
			config.OutputFormat = yml.OutputFormatJSON
		} else {
			config.OutputFormat = yml.OutputFormatYAML
		}
		core.SetConfig(config)
	}

	return openapi.Marshal(ctx, doc, t.w)
}

// syncDoc syncs high-level model changes to YAML nodes in-memory
func syncDoc(ctx context.Context, doc *openapi.OpenAPI) error {
	return openapi.Sync(ctx, doc)
}

// reloadFromYAML marshals the YAML node directly and re-parses to create a fresh document.
// Use this after modifying YAML nodes directly to ensure high-level and YAML are in sync.
func reloadFromYAML(ctx context.Context, root *yaml.Node) (*openapi.OpenAPI, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}

	doc, _, err := openapi.Unmarshal(ctx, &buf, openapi.WithSkipValidation())
	if err != nil {
		return nil, err
	}

	return doc, nil
}
