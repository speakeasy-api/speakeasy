package transform

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"gopkg.in/yaml.v2"
)

func ConvertSwagger(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	input, err := os.ReadFile(schemaPath)
	if err != nil {
		panic(err)
	}

	var format = filepath.Ext(schemaPath)
	var swaggerDoc openapi2.T

	switch format {
	case ".json":
		if err = json.Unmarshal(input, &swaggerDoc); err != nil {
			panic(err)
		}
	case ".yaml":
		if err = yaml.Unmarshal(input, &swaggerDoc); err != nil {
			panic(err)
		}
	}

	openapiSpec, err := openapi2conv.ToV3(&swaggerDoc)
	if err != nil {
		panic(err)
	}

	if yamlOut {
		enc := yaml.NewEncoder(w)
		enc.Encode(openapiSpec)
	} else {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(openapiSpec)
	}

	return nil
}
