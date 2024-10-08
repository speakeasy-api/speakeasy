package overlay

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
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

	bytes, err := schemas.Render(ys, schema, yamlOut)
	if err != nil {
		return fmt.Errorf("failed to Render document: %w", err)
	}

	if _, err := w.Write(bytes); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	return nil
}
