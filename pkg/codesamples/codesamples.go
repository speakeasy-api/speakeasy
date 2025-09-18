package codesamples

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/AlekSi/pointer"
	"github.com/speakeasy-api/openapi-overlay/pkg/overlay"
	"github.com/speakeasy-api/openapi/yml"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/usagegen"
	"gopkg.in/yaml.v3"
)

type CodeSamplesStyle int

const (
	Default CodeSamplesStyle = iota
	ReadMe
)

type CodeSampleExampleSource struct {
	// Map from header/param name to value
	Params map[string]string
	// JSON string of the request body
	RequestBodyJSON string
}

func GenerateOverlay(ctx context.Context, schema, header, token, configPath, overlayFilename string, langs []string, isWorkflow bool, isSilent bool, opts workflow.CodeSamples) (string, error) {
	targetToCodeSamples := map[string][]usagegen.UsageSnippet{}

	if isSilent {
		logger := log.From(ctx)
		var logs bytes.Buffer
		logCapture := logger.WithWriter(&logs)
		ctx = log.With(ctx, logCapture)
	}

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
			nil,
			nil,
		); err != nil {
			return "", err
		}

		log.From(ctx).Infof("\nGenerated usage snippets for %s\n\n", lang)

		snippets, err := usagegen.ParseUsageOutput(lang, usageOutput.String())
		if err != nil {
			return "", err
		}

		for _, snippet := range snippets {
			target := overlay.NewTargetSelector(snippet.Path, snippet.Method)

			targetToCodeSamples[target] = append(targetToCodeSamples[target], snippet)
		}
	}

	var actions []overlay.Action
	targets := []string{}
	for target := range targetToCodeSamples {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	for _, target := range targets {
		snippets := targetToCodeSamples[target]
		actions = append(actions, overlay.Action{
			Target: target,
			Update: *rootCodeSampleNode(ctx, snippets, opts),
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

	if overlayFilename != "" {
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

// GenerateUsageSnippet us used in the Temporal job in SpeakeasyRegistry
func GenerateUsageSnippet(ctx context.Context, schema, header, token, configPath, lang string, isSilent bool, operationID *string, example *CodeSampleExampleSource) ([]usagegen.UsageSnippet, error) {
	if isSilent {
		logger := log.From(ctx)
		var logs bytes.Buffer
		logCapture := logger.WithWriter(&logs)
		ctx = log.With(ctx, logCapture)
	}

	specifiedOperation := ""
	if operationID != nil {
		specifiedOperation = *operationID
		println("specifiedOperation", specifiedOperation)
	}

	exampleParams := map[string]string{}
	if example != nil {
		exampleParams = example.Params
	}
	var exampleRequestBody *string
	if example != nil {
		exampleRequestBody = &example.RequestBodyJSON
	}

	usageOutput := &bytes.Buffer{}
	if err := usagegen.Generate(
		ctx,
		config.GetCustomerID(),
		lang,
		schema,
		header,
		token,
		"",
		specifiedOperation,
		"",
		filepath.Join(configPath, "speakeasyusagegen"),
		specifiedOperation == "",
		usageOutput,
		exampleParams,
		exampleRequestBody,
	); err != nil {
		return nil, err
	}

	log.From(ctx).Infof("\nGenerated usage snippets for %s\n\n", lang)

	snippets, err := usagegen.ParseUsageOutput(lang, usageOutput.String())
	if err != nil {
		return nil, err
	}
	return snippets, nil
}

func getStyle(opts workflow.CodeSamples) CodeSamplesStyle {
	if opts.Style != nil {
		switch *opts.Style {
		case "readme":
			return ReadMe
		}
	}
	return Default
}

func rootCodeSampleNode(ctx context.Context, snippets []usagegen.UsageSnippet, opts workflow.CodeSamples) *yaml.Node {
	var content []*yaml.Node
	for _, snippet := range snippets {
		content = append(content, singleCodeSampleNode(ctx, snippet, opts))
	}

	switch getStyle(opts) {
	case Default:
		return yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("x-codeSamples"), yml.CreateOrUpdateSliceNode(ctx, content, nil)})
	case ReadMe:
		return yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("x-readme"), yml.CreateMapNode(ctx, []*yaml.Node{yml.CreateStringNode("code-samples"), yml.CreateOrUpdateSliceNode(ctx, content, nil)})})
	}

	panic("unrecognized style")
}

func singleCodeSampleNode(ctx context.Context, snippet usagegen.UsageSnippet, opts workflow.CodeSamples) *yaml.Node {
	lang := snippet.Language
	if opts.LangOverride != nil {
		lang = *opts.LangOverride
	}

	label := pointer.ToString(snippet.OperationId)
	if opts.LabelOverride != nil {
		if opts.LabelOverride.Omit != nil {
			if *opts.LabelOverride.Omit {
				label = nil
			}
		} else if opts.LabelOverride.FixedValue != nil {
			label = opts.LabelOverride.FixedValue
		}
	}

	var kvs []*yaml.Node
	switch getStyle(opts) {
	case Default:
		kvs = append(kvs, yml.CreateStringNode("lang"), yml.CreateStringNode(lang))
		if label != nil {
			kvs = append(kvs, yml.CreateStringNode("label"), yml.CreateStringNode(*label))
		}
		kvs = append(kvs, yml.CreateStringNode("source"), yml.CreateStringNode(snippet.Snippet))
	case ReadMe:
		if label != nil {
			kvs = append(kvs, yml.CreateStringNode("name"), yml.CreateStringNode(*label))
		}
		kvs = append(kvs, yml.CreateStringNode("language"), yml.CreateStringNode(lang), yml.CreateStringNode("code"), yml.CreateStringNode(snippet.Snippet))
	}

	return yml.CreateMapNode(ctx, kvs)
}
