package run

import (
	"context"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

func TestPrepareWorkflowContextEnsuresPlatformForRegistryWorkflows(t *testing.T) { //nolint:paralleltest
	origTargets, origPlatform := ensureTargets, ensurePlatform
	t.Cleanup(func() { ensureTargets, ensurePlatform = origTargets, origPlatform })
	ensureTargets = func(ctx context.Context, _ []string) (context.Context, error) { return ctx, nil }

	blockingFalse := false
	registryInputSource := workflow.Source{
		Inputs: []workflow.Document{{Location: "registry.speakeasyapi.dev/org/ws/ns"}},
	}
	localSource := workflow.Source{
		Inputs: []workflow.Document{{Location: "openapi.yaml"}},
	}

	tests := []struct {
		name         string
		workflow     *Workflow
		wantPlatform bool
	}{
		{
			name:         "frozen workflow lock",
			workflow:     &Workflow{FrozenWorkflowLock: true},
			wantPlatform: true,
		},
		{
			name: "selected registry input source",
			workflow: &Workflow{Source: "s", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": registryInputSource},
			}},
			wantPlatform: true,
		},
		{
			name: "selected target with a registry input source",
			workflow: &Workflow{Target: "t", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": registryInputSource},
				Targets: map[string]workflow.Target{"t": {Target: "go", Source: "s"}},
			}},
			wantPlatform: true,
		},
		{
			name: "registry overlay",
			workflow: &Workflow{Source: "s", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": {Overlays: []workflow.Overlay{{Document: &workflow.Document{Location: "registry.speakeasyapi.dev/org/ws/ns"}}}}},
			}},
			wantPlatform: true,
		},
		{
			name: "blocking code samples registry",
			workflow: &Workflow{Target: "t", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": localSource},
				Targets: map[string]workflow.Target{"t": {Target: "go", Source: "s", CodeSamples: &workflow.CodeSamples{Registry: &workflow.SourceRegistry{}}}},
			}},
			wantPlatform: true,
		},
		{
			name: "non-blocking code samples registry",
			workflow: &Workflow{Target: "t", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": localSource},
				Targets: map[string]workflow.Target{"t": {Target: "go", Source: "s", CodeSamples: &workflow.CodeSamples{Registry: &workflow.SourceRegistry{}, Blocking: &blockingFalse}}},
			}},
			wantPlatform: false,
		},
		{
			name: "best-effort source publishing",
			workflow: &Workflow{Target: "t", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": {Inputs: localSource.Inputs, Registry: &workflow.SourceRegistry{}}},
				Targets: map[string]workflow.Target{"t": {Target: "go", Source: "s"}},
			}},
			wantPlatform: false,
		},
		{
			name: "unselected registry source",
			workflow: &Workflow{Target: "t", workflow: workflow.Workflow{
				Sources: map[string]workflow.Source{"s": localSource, "other": registryInputSource},
				Targets: map[string]workflow.Target{"t": {Target: "go", Source: "s"}},
			}},
			wantPlatform: false,
		},
	}
	for _, tt := range tests { //nolint:paralleltest
		t.Run(tt.name, func(t *testing.T) {
			called := false
			ensurePlatform = func(ctx context.Context) (context.Context, error) {
				called = true
				return ctx, nil
			}
			if _, err := tt.workflow.prepareWorkflowContext(context.Background()); err != nil {
				t.Fatalf("prepareWorkflowContext: %v", err)
			}
			if called != tt.wantPlatform {
				t.Fatalf("ensurePlatform called = %v, want %v", called, tt.wantPlatform)
			}
		})
	}
}
