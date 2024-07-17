package overlay

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pb33f/libopenapi/json"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy-core/openapi"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
	"io"
)

func Validate(overlayFile string) error {
	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	return o.Validate()
}

func Compare(schemas []string, w io.Writer) error {
	if len(schemas) != 2 {
		return fmt.Errorf("Exactly two --schemas must be passed to perform a comparison.")
	}

	y1, err := loader.LoadSpecification(schemas[0])
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", schemas[0], err)
	}

	y2, err := loader.LoadSpecification(schemas[1])
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", schemas[1], err)
	}

	title := fmt.Sprintf("Overlay %s => %s", schemas[0], schemas[1])

	o, err := overlay.Compare(title, y1, *y2)
	if err != nil {
		return fmt.Errorf("failed to compare spec files %q and %q: %w", schemas[0], schemas[1], err)
	}

	if err := o.Format(w); err != nil {
		return fmt.Errorf("failed to format overlay: %w", err)
	}

	return nil
}

func Apply(schema string, overlayFile string, yamlOut bool, w io.Writer, strict bool, warn bool) error {
	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	if err := o.Validate(); err != nil {
		return err
	}

	ys, specFile, err := loader.LoadEitherSpecification(schema, o)
	if err != nil {
		return err
	}

	if strict {
		err, warnings := o.ApplyToStrict(ys)
		for _, warning := range warnings {
			log.From(context.Background()).Warnf("WARN: %s", warning)
		}
		if err != nil {
			return fmt.Errorf("failed to apply overlay to spec file (strict) %q: %w", specFile, err)
		}
	} else {
		if err := o.ApplyTo(ys); err != nil {
			return fmt.Errorf("failed to apply overlay to spec file %q: %w", specFile, err)
		}
	}

	bytes, err := render(ys, schema, yamlOut)
	if err != nil {
		return fmt.Errorf("failed to render document: %w", err)
	}

	if _, err := w.Write(bytes); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	return nil
}

func render(y *yaml.Node, schemaPath string, yamlOut bool) ([]byte, error) {
	yamlIn := utils.HasYAMLExt(schemaPath)

	if yamlIn && yamlOut {
		var res bytes.Buffer
		encoder := yaml.NewEncoder(&res)
		// Note: would love to make this generic but the indentation information isn't in go-yaml nodes
		// https://github.com/go-yaml/yaml/issues/899
		encoder.SetIndent(2)
		if err := encoder.Encode(y); err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		return res.Bytes(), nil
	}

	// Preserves key ordering
	specBytes, err := json.YAMLNodeToJSON(y, "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	if yamlOut {
		// Use libopenapi to convert JSON to YAML to preserve key ordering
		_, model, err := openapi.Load(specBytes, schemaPath)

		yamlBytes, err := model.Model.Render()
		if err != nil {
			return nil, fmt.Errorf("failed to render YAML: %w", err)
		}

		return yamlBytes, nil
	} else {
		return specBytes, nil
	}
}
