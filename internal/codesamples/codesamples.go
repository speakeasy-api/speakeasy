package codesamples

import (
	"bytes"
	"context"
	"fmt"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"github.com/speakeasy-api/speakeasy/internal/yamlutil"
	"os"
	"path/filepath"

	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"gopkg.in/yaml.v3"
)

type CodeSamplesStyle int

const (
	Default CodeSamplesStyle = iota
	ReadMe
)

func GenerateOverlay(ctx context.Context, schema, header, token, configPath, overlayFilename string, langs []string, isWorkflow bool, style CodeSamplesStyle) (string, error) {
	targetToCodeSamples := map[string][]usagegen.UsageSnippet{}
	isJSON := filepath.Ext(schema) == ".json"

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
			filepath.Join(configPath, "speakeasyusagegen"),
			true,
			usageOutput,
		); err != nil {
			return "", err
		}

		log.From(ctx).Infof("\nGenerated usage snippets for %s\n\n", lang)

		snippets, err := usagegen.ParseUsageOutput(lang, usageOutput.String())
		if err != nil {
			return "", err
		}

		for _, snippet := range snippets {
			target := fmt.Sprintf(`$["paths"]["%s"]["%s"]`, snippet.Path, snippet.Method)

			targetToCodeSamples[target] = append(targetToCodeSamples[target], snippet)
		}
	}

	var actions []overlay.Action
	for target, snippets := range targetToCodeSamples {
		actions = append(actions, overlay.Action{
			Target: target,
			Update: *rootCodeSampleNode(snippets, style, isJSON),
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
		return "", err
	}

	if overlayFilename == "" {
		println(overlayString)
	} else {
		f, err := os.Create(overlayFilename)
		if err != nil {
			return overlayString, err
		}
		defer f.Close()

		if _, err = f.WriteString(overlayString); err != nil {
			return overlayString, err
		}
	}

	return overlayString, nil
}

func rootCodeSampleNode(snippets []usagegen.UsageSnippet, style CodeSamplesStyle, isJSON bool) *yaml.Node {
	builder := yamlutil.NewBuilder(isJSON)

	var content []*yaml.Node
	for _, snippet := range snippets {
		content = append(content, singleCodeSampleNode(snippet, style, builder))
	}

	switch style {
	case Default:
		return builder.NewListNode("x-codeSamples", content)
	case ReadMe:
		return builder.NewNode("x-readme", builder.NewListNode("code-samples", content))
	}

	panic("unrecognized style")
}

func singleCodeSampleNode(snippet usagegen.UsageSnippet, style CodeSamplesStyle, builder *yamlutil.Builder) *yaml.Node {
	switch style {
	case Default:
		return builder.NewMultinode("lang", snippet.Language, "label", snippet.OperationId, "source", snippet.Snippet)
	case ReadMe:
		return builder.NewMultinode("name", snippet.OperationId, "language", snippet.Language, "code", snippet.Snippet)
	}

	panic("unrecognized style")
}

func styleForNode(isJSON bool) yaml.Style {
	if isJSON {
		return yaml.DoubleQuotedStyle
	}

	return 0
}
