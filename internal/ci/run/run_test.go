package run_test

import (
	"reflect"
	"testing"

	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/ci/run"
)

func ptr[T any](v T) *T {
	return &v
}

func TestAddTargetPublishOutputs(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		target          workflow.Target
		installationURL *string
		expectedOutputs map[string]string
	}{
		"csharp-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "csharp",
			},
			expectedOutputs: map[string]string{
				"publish_csharp": "false",
			},
		},
		"csharp-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					Nuget: &workflow.Nuget{
						APIKey: "non-empty",
					},
				},
				Target: "csharp",
			},
			expectedOutputs: map[string]string{
				"publish_csharp": "true",
			},
		},
		"go-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config, Go does not use
				Target:     "go",
			},
			expectedOutputs: map[string]string{
				"publish_go": "true",
			},
		},
		"java-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "java",
			},
			expectedOutputs: map[string]string{
				"publish_java": "false",
			},
		},
		"java-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					Java: &workflow.Java{
						GPGSecretKey:  "non-empty",
						GPGPassPhrase: "non-empty",
						OSSHRPassword: "non-empty",
						OSSRHUsername: "non-empty",
					},
				},
				Target: "java",
			},
			expectedOutputs: map[string]string{
				"publish_java":        "true",
				"use_sonatype_legacy": "false",
			},
		},
		"mcp-typescript-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "mcp-typescript",
			},
			expectedOutputs: map[string]string{
				"publish_mcp_typescript": "false",
			},
		},
		"mcp-typescript-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					NPM: &workflow.NPM{
						Token: "non-empty",
					},
				},
				Target: "mcp-typescript",
			},
			expectedOutputs: map[string]string{
				"publish_mcp_typescript": "true",
			},
		},
		"php-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "php",
			},
			expectedOutputs: map[string]string{
				"publish_php": "false",
			},
		},
		"php-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					Packagist: &workflow.Packagist{
						Token:    "non-empty",
						Username: "non-empty",
					},
				},
				Target: "php",
			},
			expectedOutputs: map[string]string{
				"publish_php": "true",
			},
		},
		"python-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "python",
			},
			expectedOutputs: map[string]string{
				"publish_python": "false",
			},
		},
		"python-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					PyPi: &workflow.PyPi{
						Token: "non-empty",
					},
				},
				Target: "python",
			},
			expectedOutputs: map[string]string{
				"publish_python": "true",
			},
		},
		"python-trusted-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					PyPi: &workflow.PyPi{
						UseTrustedPublishing: ptr(true),
					},
				},
				Target: "python",
			},
			expectedOutputs: map[string]string{
				"publish_python":              "true",
				"use_pypi_trusted_publishing": "true",
			},
		},
		"ruby-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "ruby",
			},
			expectedOutputs: map[string]string{
				"publish_ruby": "false",
			},
		},
		"ruby-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					RubyGems: &workflow.RubyGems{
						Token: "non-empty",
					},
				},
				Target: "ruby",
			},
			expectedOutputs: map[string]string{
				"publish_ruby": "true",
			},
		},
		"terraform-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "terraform",
			},
			expectedOutputs: map[string]string{
				"publish_terraform": "false",
			},
		},
		"terraform-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					Terraform: &workflow.Terraform{
						GPGPrivateKey: "non-empty",
						GPGPassPhrase: "non-empty",
					},
				},
				Target: "terraform",
			},
			expectedOutputs: map[string]string{
				"publish_terraform": "true",
			},
		},
		"typescript-no-publishing": {
			target: workflow.Target{
				Publishing: nil, // intentionally no publishing config
				Target:     "typescript",
			},
			expectedOutputs: map[string]string{
				"publish_typescript": "false",
			},
		},
		"typescript-publishing": {
			target: workflow.Target{
				Publishing: &workflow.Publishing{
					NPM: &workflow.NPM{
						Token: "non-empty",
					},
				},
				Target: "typescript",
			},
			expectedOutputs: map[string]string{
				"publish_typescript": "true",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// AddTargetPublishOutputs is only additive currently.
			gotOutputs := make(map[string]string)
			run.AddTargetPublishOutputs(tc.target, gotOutputs, tc.installationURL)

			// Check if the outputs match the expected outputs
			if !reflect.DeepEqual(gotOutputs, tc.expectedOutputs) {
				t.Errorf("expected %v, got %v", tc.expectedOutputs, gotOutputs)
			}
		})
	}
}
