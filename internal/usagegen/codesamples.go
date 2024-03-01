package usagegen

import (
	"bytes"
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

func GenerateCodeSamplesOverlay(ctx context.Context, schema, header, token, configPath, overlayFilename string, langs []string) error {
	targetToCodeSamples := map[string][]UsageSnippet{}

	for _, lang := range langs {
		usageOutput := &bytes.Buffer{}

		if err := Generate(
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

		snippets, err := ParseUsageOutput(lang, usageOutput.String())
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
