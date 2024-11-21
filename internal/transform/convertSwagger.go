package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"gopkg.in/yaml.v2"
)

func ConvertSwagger(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	return transformer[interface{}]{
		schemaPath:  schemaPath,
		transformFn: convertSwaggerDoc,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func ConvertSwaggerFromReader(ctx context.Context, schema io.Reader, schemaPath string, w io.Writer, yamlOut bool) error {
	return transformer[interface{}]{
		r:           schema,
		schemaPath:  schemaPath,
		transformFn: convertSwaggerDoc,
		w:           w,
		jsonOut:     !yamlOut,
	}.Do(ctx)
}

func convertSwaggerDoc(ctx context.Context, doc libopenapi.Document, model *libopenapi.DocumentModel[v3.Document], _ interface{}) (libopenapi.Document, *libopenapi.DocumentModel[v3.Document], error) {
	root := model.Index.GetRootNode()

	rawDoc, err := yaml.Marshal(root)
	if err != nil {
		return doc, model, fmt.Errorf("failed to marshal document: %w", err)
	}

	var swaggerDoc openapi2.T
	if err := yaml.Unmarshal(rawDoc, &swaggerDoc); err != nil {
		return doc, model, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	openapiSpec, err := openapi2conv.ToV3(&swaggerDoc)
	if err != nil {
		return doc, model, err
	}

	rawDoc, err = json.MarshalIndent(openapiSpec, "", "  ")
	if err != nil {
		return doc, model, fmt.Errorf("failed to marshal document: %w", err)
	}

	// Load the converted spec into a libopenapi document
	docNew, model, err := openapi.Load(rawDoc, doc.GetConfiguration().BasePath)
	if err != nil {
		return doc, model, err
	}

	return *docNew, model, nil
}
