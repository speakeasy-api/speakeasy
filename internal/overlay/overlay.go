package overlay

import (
	"bytes"
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/loader"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
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

	o, err := overlay.Compare(title, schemas[0], y1, *y2)
	if err != nil {
		return fmt.Errorf("failed to compare spec files %q and %q: %w", schemas[0], schemas[1], err)
	}

	if err := o.Format(os.Stdout); err != nil {
		return fmt.Errorf("failed to format overlay: %w", err)
	}

	return nil
}

func Apply(schema string, overlayFile string, w io.Writer) error {
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

	if err := o.ApplyTo(ys); err != nil {
		return fmt.Errorf("failed to apply overlay to spec file %q: %w", specFile, err)
	}

	enc := yaml.NewEncoder(w)
	if err := enc.Encode(ys); err != nil {
		return fmt.Errorf("failed to encode spec file %q: %w", specFile, err)
	}

	return nil
}

func CodeSamples(ctx context.Context, schema, header, token, configPath, overlayFilename string, langs []string) error {
	targetToCodeSamples := map[string][]usagegen.UsageSnippet{}

	for _, lang := range langs {
		usageOutput := &bytes.Buffer{}

		if err := usagegen.Generate(
			ctx,
			config.GetCustomerID(),
			lang,
			schema,
			header,
			token,
			"",
			"",
			"",
			configPath,
			true,
			usageOutput,
		); err != nil {
			return err
		}

		log.From(ctx).Infof("\nGenerated usage snippets for %s\n\n", lang)

		snippets, err := usagegen.ParseUsageOutput(lang, usageOutput.String())
		if err != nil {
			return err
		}

		for _, snippet := range snippets {
			target := fmt.Sprintf(`$["paths"]["%s"]["%s"]`, snippet.Path, snippet.Method)

			targetToCodeSamples[target] = append(targetToCodeSamples[target], snippet)
		}
	}

	var actions []overlay.Action
	for target, snippets := range targetToCodeSamples {
		var content []*yaml.Node
		for _, snippet := range snippets {
			content = append(content,
				&yaml.Node{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "lang"},
						{Kind: yaml.ScalarNode, Value: snippet.Language},
						{Kind: yaml.ScalarNode, Value: "label"},
						{Kind: yaml.ScalarNode, Value: snippet.OperationId},
						{Kind: yaml.ScalarNode, Value: "source"},
						{Kind: yaml.ScalarNode, Value: snippet.Snippet},
					},
				})
		}

		actions = append(actions, overlay.Action{
			Target: target,
			Update: yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "x-codeSamples"},
					{
						Kind:    yaml.SequenceNode,
						Content: content,
					},
				},
			},
		})
	}

	extends := schema
	abs, err := filepath.Abs(schema)
	if err == nil {
		extends = "file://" + abs
	}

	overlay := &overlay.Overlay{
		Version: "1.0.0",
		Info: overlay.Info{
			Title:   fmt.Sprintf("CodeSamples overlay for %s", schema),
			Version: "0.0.0",
		},
		Extends: extends,
		Actions: actions,
	}

	overlayString, err := overlay.ToString()
	if err != nil {
		return err
	}

	if overlayFilename == "" {
		println(overlayString)
	} else {
		f, err := os.Create(overlayFilename)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err = f.WriteString(overlayString); err != nil {
			return err
		}
	}

	return nil
}
