package run

import (
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/stretchr/testify/require"
)

func TestWorkflowSourceHasRemoteInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		source   workflow.Source
		expected bool
	}{
		{
			name: "local file input",
			source: workflow.Source{
				Inputs: []workflow.Document{{Location: "./openapi.yaml"}},
			},
			expected: false,
		},
		{
			name: "remote HTTPS input",
			source: workflow.Source{
				Inputs: []workflow.Document{{Location: "https://example.com/openapi.yaml"}},
			},
			expected: true,
		},
		{
			name: "registry input",
			source: workflow.Source{
				Inputs: []workflow.Document{{Location: "registry.speakeasyapi.dev/org/workspace/spec"}},
			},
			expected: true,
		},
		{
			name: "mixed local and remote",
			source: workflow.Source{
				Inputs: []workflow.Document{
					{Location: "./local.yaml"},
					{Location: "https://example.com/remote.yaml"},
				},
			},
			expected: true,
		},
		{
			name: "mixed local and registry",
			source: workflow.Source{
				Inputs: []workflow.Document{
					{Location: "./local.yaml"},
					{Location: "registry.speakeasyapi.dev/org/workspace/spec"},
				},
			},
			expected: true,
		},
		{
			name: "empty inputs",
			source: workflow.Source{
				Inputs: []workflow.Document{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := workflowSourceHasRemoteInputs(tt.source)
			require.Equal(t, tt.expected, result)
		})
	}
}
