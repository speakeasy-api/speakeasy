package run

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/workflowTracking"
	"github.com/speakeasy-api/speakeasy/registry"
)

type FrozenSource struct {
	workflow   *Workflow
	parentStep *workflowTracking.WorkflowStep
	sourceID   string
}

var _ SourceStep = FrozenSource{}

func NewFrozenSource(w *Workflow, parentStep *workflowTracking.WorkflowStep, sourceID string) FrozenSource {
	return FrozenSource{
		workflow:   w,
		parentStep: parentStep,
		sourceID:   sourceID,
	}
}

func (f FrozenSource) Do(ctx context.Context, _ string) (string, error) {
	mergeStep := f.parentStep.NewSubstep("Download OAS from lockfile")

	// Check lockfile exists, produce an error if not
	if f.workflow.lockfileOld == nil {
		return "", fmt.Errorf("workflow lacks a prior lock file: can't use this on first run")
	}
	lockSource, ok := f.workflow.lockfileOld.Sources[f.sourceID]
	if !ok {
		return "", fmt.Errorf("workflow lockfile lacks a reference to source %s: can't use this on first run", f.sourceID)
	}
	if !registry.IsRegistryEnabled(ctx) {
		return "", fmt.Errorf("registry is not enabled for this workspace")
	}
	if lockSource.SourceBlobDigest == "" || lockSource.SourceRevisionDigest == "" || lockSource.SourceNamespace == "" {
		return "", fmt.Errorf("invalid workflow lockfile: namespace = %s blobDigest = %s revisionDigest = %s", lockSource.SourceNamespace, lockSource.SourceBlobDigest, lockSource.SourceRevisionDigest)
	}

	var registryBreakdown *workflow.SpeakeasyRegistryDocument
	var err error

	if isSingleRegistrySource(f.workflow.workflow.Sources[f.sourceID]) && f.workflow.workflow.Sources[f.sourceID].Registry == nil {
		d := f.workflow.workflow.Sources[f.sourceID].Inputs[0]
		registryBreakdown = workflow.ParseSpeakeasyRegistryReference(d.Location.Resolve())
		if registryBreakdown == nil {
			return "", fmt.Errorf("failed to parse speakeasy registry reference %s", d.Location)
		}
	} else if !isSingleRegistrySource(f.workflow.workflow.Sources[f.sourceID]) && f.workflow.workflow.Sources[f.sourceID].Registry == nil {
		return "", fmt.Errorf("invalid workflow lockfile: no registry location found for source %s", f.sourceID)
	} else if f.workflow.workflow.Sources[f.sourceID].Registry != nil {
		registryBreakdown = f.workflow.workflow.Sources[f.sourceID].Registry.Location.Parse()
		if registryBreakdown == nil {
			return "", fmt.Errorf("error parsing registry location %s: %w", string(f.workflow.workflow.Sources[f.sourceID].Registry.Location), err)
		}
	}

	if lockSource.SourceNamespace != registryBreakdown.NamespaceName {
		return "", fmt.Errorf("invalid workflow lockfile: namespace %s != %s", lockSource.SourceNamespace, registryBreakdown.NamespaceID)
	}

	registryBreakdown.Reference = lockSource.SourceRevisionDigest
	registryLocation := registryBreakdown.MakeURL(true)

	d := workflow.Document{Location: workflow.LocationString(registryLocation)}
	docPath, err := registry.ResolveSpeakeasyRegistryBundle(ctx, d, workflow.GetTempDir())

	if err != nil {
		return "", fmt.Errorf("error resolving registry bundle from %s: %w", registryLocation, err)
	}

	mergeStep.Succeed()

	return docPath.LocalFilePath, nil
}
