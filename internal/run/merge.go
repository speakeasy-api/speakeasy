package run

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schemas"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/pkg/merge"
	"os"
	"path/filepath"
)

type Merge struct {
	workflow   *Workflow
	parentStep *workflowTracking.WorkflowStep
	source     workflow.Source
	ruleset    string
}

var _ SourceStep = Merge{}

func NewMerge(w *Workflow, parentStep *workflowTracking.WorkflowStep, source workflow.Source, ruleset string) Merge {
	return Merge{
		workflow:   w,
		parentStep: parentStep,
		source:     source,
		ruleset:    ruleset,
	}
}

func (m Merge) Do(ctx context.Context, _ string) (string, error) {
	mergeStep := m.parentStep.NewSubstep("Merge Documents")

	mergeLocation := m.source.GetTempMergeLocation()

	log.From(ctx).Infof("Merging %d schemas into %s...", len(m.source.Inputs), mergeLocation)

	inSchemas := []string{}
	for _, input := range m.source.Inputs {
		resolvedPath, err := schemas.ResolveDocument(ctx, input, nil, mergeStep)
		if err != nil {
			return "", err
		}
		inSchemas = append(inSchemas, resolvedPath)
	}

	mergeStep.NewSubstep(fmt.Sprintf("Merge %d documents", len(m.source.Inputs)))

	if err := mergeDocuments(ctx, inSchemas, mergeLocation, m.ruleset, m.workflow.ProjectDir, m.workflow.SkipGenerateLintReport); err != nil {
		return "", err
	}

	return mergeLocation, nil
}

func mergeDocuments(ctx context.Context, inSchemas []string, outFile, defaultRuleset, workingDir string, skipGenerateLintReport bool) error {
	if err := os.MkdirAll(filepath.Dir(outFile), os.ModePerm); err != nil {
		return err
	}

	if err := merge.MergeOpenAPIDocuments(ctx, inSchemas, outFile, defaultRuleset, workingDir, skipGenerateLintReport); err != nil {
		return err
	}

	log.From(ctx).Printf("Successfully merged %d schemas into %s", len(inSchemas), outFile)

	return nil
}
