package overlay

import (
	"context"
	"fmt"
	"io"

	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

type ChangeType int

const (
	Update ChangeType = iota
	Remove
)

type Summary struct {
	TargetToChangeType map[string]ChangeType
}

func Validate(overlayFile string) error {
	o, err := loader.LoadOverlay(overlayFile)
	if err != nil {
		return err
	}

	return o.Validate()
}

func Compare(schemas []string, w io.Writer) (*Summary, error) {
	if len(schemas) != 2 {
		return nil, fmt.Errorf("Exactly two --schemas must be passed to perform a comparison.")
	}

	y1, err := loader.LoadSpecification(schemas[0])
	if err != nil {
		return nil, fmt.Errorf("failed to load %q: %w", schemas[0], err)
	}

	y2, err := loader.LoadSpecification(schemas[1])
	if err != nil {
		return nil, fmt.Errorf("failed to load %q: %w", schemas[1], err)
	}

	title := fmt.Sprintf("Overlay %s => %s", schemas[0], schemas[1])

	o, err := overlay.Compare(title, y1, *y2)
	if err != nil {
		return nil, fmt.Errorf("failed to compare spec files %q and %q: %w", schemas[0], schemas[1], err)
	}

	if err := o.Format(w); err != nil {
		return nil, fmt.Errorf("failed to format overlay: %w", err)
	}

	return Summarize(o), nil
}

func Apply(schema string, overlayFiles []string, yamlOut bool, w io.Writer, strict bool, warn bool) (*Summary, error) {
	var summary *Summary
	var err error

	// Load the initial schema
	ys, specFile, err := loader.LoadEitherSpecification(schema, nil)
	if err != nil {
		return nil, err
	}

	for _, overlayFile := range overlayFiles {
		o, err := loader.LoadOverlay(overlayFile)
		if err != nil {
			return nil, err
		}

		summary, err = Summarize(o), ApplyWithSourceLocation(ys, o, specFile, yamlOut, w, strict)
		if err != nil {
			return nil, err
		}
	}

	return summary, nil
}

func Summarize(o *overlay.Overlay) *Summary {
	targets := make(map[string]ChangeType)

	for _, action := range o.Actions {
		changeType := Update
		if action.Remove {
			changeType = Remove
		}
		targets[action.Target] = changeType
	}

	return &Summary{
		TargetToChangeType: targets,
	}
}

func ApplyWithSourceLocation(document *yaml.Node, o *overlay.Overlay, sourceLocation string, yamlOut bool, w io.Writer, strict bool) error {
	return apply(document, o, sourceLocation, utils.HasYAMLExt(sourceLocation), yamlOut, w, strict)
}

// ApplyDirect is used by the Registry to apply overlays from registry-based documents which do not have local file references
func ApplyDirect(document *yaml.Node, o *overlay.Overlay, yamlIn, yamlOut bool, w io.Writer, strict bool) error {
	return apply(document, o, "", yamlIn, yamlOut, w, strict)
}

func apply(document *yaml.Node, o *overlay.Overlay, sourceLocation string, yamlIn, yamlOut bool, w io.Writer, strict bool) error {
	if err := o.Validate(); err != nil {
		return err
	}

	if strict {
		err, warnings := o.ApplyToStrict(document)
		for _, warning := range warnings {
			log.From(context.Background()).Warnf("WARN: %s", warning)
		}
		if err != nil {
			return fmt.Errorf("failed to apply overlay to spec file (strict): %w", err)
		}
	} else {
		if err := o.ApplyTo(document); err != nil {
			return fmt.Errorf("failed to apply overlay to spec file: %w", err)
		}
	}

	bytes, err := schemas.RenderDocument(document, sourceLocation, yamlIn, yamlOut)
	if err != nil {
		return fmt.Errorf("failed to Render document: %w", err)
	}

	if _, err := w.Write(bytes); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	return nil
}
