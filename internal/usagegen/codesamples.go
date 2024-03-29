package usagegen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
)

func GenerateCodeSamplesOverlay(ctx context.Context, schema, header, token, configPath, overlayFilename string, langs []string, isWorkflow bool) error {
	targetToCodeSamples := map[string][]UsageSnippet{}
	isJSON := filepath.Ext(schema) == ".json"

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
						{Kind: yaml.ScalarNode, Value: "lang", Style: styleForNode(isJSON)},
						{Kind: yaml.ScalarNode, Value: snippet.Language, Style: styleForNode(isJSON)},
						{Kind: yaml.ScalarNode, Value: "label", Style: styleForNode(isJSON)},
						{Kind: yaml.ScalarNode, Value: snippet.OperationId, Style: styleForNode(isJSON)},
						{Kind: yaml.ScalarNode, Value: "source", Style: styleForNode(isJSON)},
						{Kind: yaml.ScalarNode, Value: snippet.Snippet},
					},
				})
		}

		actions = append(actions, overlay.Action{
			Target: target,
			Update: yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "x-codeSamples", Style: styleForNode(isJSON)},
					{
						Kind:    yaml.SequenceNode,
						Content: content,
					},
				},
			},
		})
	}

	extends := schema
	title := fmt.Sprintf("CodeSamples overlay for %s", schema)
	abs, err := filepath.Abs(schema)
	if err == nil {
		extends = "file://" + abs
	}

	if isWorkflow {
		title = fmt.Sprintf("CodeSamples overlay for %s target", langs[0])
	}

	overlay := &overlay.Overlay{
		Version: "1.0.0",
		Info: overlay.Info{
			Title:   title,
			Version: "0.0.0",
		},
		Actions: actions,
	}

	if !isWorkflow {
		overlay.Extends = extends
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

func styleForNode(isJSON bool) yaml.Style {
	if isJSON {
		return yaml.DoubleQuotedStyle
	}

	return 0
}
