package transform

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/invopop/yaml"
)

func ConvertSwagger(ctx context.Context, schemaPath string, yamlOut bool, w io.Writer) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	var swaggerDoc openapi2.T
	if strings.Contains(schemaPath, ".json") {
		err = json.Unmarshal(schemaBytes, &swaggerDoc)
	} else {
		err = yaml.Unmarshal(schemaBytes, &swaggerDoc)
	}
	if err != nil {
		return err
	}

	openAPIV3Spec, err := openapi2conv.ToV3(&swaggerDoc)
	if err != nil {
		return err
	}

	var outBytes []byte
	if yamlOut {
		outBytes, err = yaml.Marshal(openAPIV3Spec)
	} else {
		outBytes, err = json.Marshal(openAPIV3Spec)
	}
	if err != nil {
		return err
	}

	_, err = w.Write(outBytes)
	if err != nil {
		return err
	}

	return nil
}
