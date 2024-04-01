package merge

import (
	"context"
	"github.com/AlekSi/pointer"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/bundler"
)

func MergeOpenAPIDocuments(ctx context.Context, inFiles []string, outFile, defaultRuleset, workingDir string) error {
	inputs := make([]workflow.Document, len(inFiles))
	for i, inFile := range inFiles {
		inputs[i] = workflow.Document{
			Location: inFile,
		}
	}
	source := workflow.Source{
		Inputs:  inputs,
		Output:  pointer.ToString(outFile),
		Ruleset: pointer.ToString(defaultRuleset),
	}

	_, err := bundler.CompileSource(ctx, nil, "", source)
	return err
}
