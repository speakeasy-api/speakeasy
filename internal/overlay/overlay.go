package overlay

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/bundler"
	"io"
	"os"
)

func Validate(overlayFile string) error {
	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	return o.Validate()
}

func Compare(schemas []string) error {
	if len(schemas) != 2 {
		return fmt.Errorf("Exactly two --schemas must be passed to perform a comparison.")
	}

	y1, err := loader.LoadSpecification(schemas[0])
	if err != nil {
		return fmt.Errorf("failed to load %q: %w", schemas[0], err)
	}

	y2, err := loader.LoadSpecification(schemas[1])
	if err != nil {
		return fmt.Errorf("failed to laod %q: %w", schemas[1], err)
	}

	title := fmt.Sprintf("Overlay %s => %s", schemas[0], schemas[1])

	o, err := overlay.Compare(title, y1, *y2)
	if err != nil {
		return fmt.Errorf("failed to compare spec files %q and %q: %w", schemas[0], schemas[1], err)
	}

	if err := o.Format(os.Stdout); err != nil {
		return fmt.Errorf("failed to format overlay: %w", err)
	}

	return nil
}

func Apply(ctx context.Context, schema string, overlayFile string, w io.Writer) error {
	source := workflow.Source{
		Inputs:   []workflow.Document{{Location: schema}},
		Overlays: []workflow.Document{{Location: overlayFile}},
	}

	return bundler.CompileSourceTo(ctx, nil, "", source, w)
}
