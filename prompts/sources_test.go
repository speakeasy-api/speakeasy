package prompts

import (
	"testing"
)

func TestExpandRegistryShorthand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full registry URI unchanged",
			input:    "registry.speakeasyapi.dev/org/workspace/namespace@latest",
			expected: "registry.speakeasyapi.dev/org/workspace/namespace@latest",
		},
		{
			name:     "shorthand with three parts expands with @latest",
			input:    "my-org/my-workspace/my-namespace",
			expected: "registry.speakeasyapi.dev/my-org/my-workspace/my-namespace@latest",
		},
		{
			name:     "shorthand with tag preserves tag",
			input:    "my-org/my-workspace/my-namespace@v1.0.0",
			expected: "registry.speakeasyapi.dev/my-org/my-workspace/my-namespace@v1.0.0",
		},
		{
			name:     "shorthand with :main tag",
			input:    "my-org/my-workspace/my-namespace:main",
			expected: "registry.speakeasyapi.dev/my-org/my-workspace/my-namespace:main",
		},
		{
			name:     "local file path unchanged",
			input:    "./openapi.yaml",
			expected: "./openapi.yaml",
		},
		{
			name:     "absolute file path unchanged",
			input:    "/path/to/openapi.yaml",
			expected: "/path/to/openapi.yaml",
		},
		{
			name:     "URL unchanged",
			input:    "https://example.com/openapi.yaml",
			expected: "https://example.com/openapi.yaml",
		},
		{
			name:     "http URL unchanged",
			input:    "http://example.com/openapi.yaml",
			expected: "http://example.com/openapi.yaml",
		},
		{
			name:     "two parts unchanged (not registry format)",
			input:    "workspace/namespace",
			expected: "workspace/namespace",
		},
		{
			name:     "four parts unchanged (not registry format)",
			input:    "a/b/c/d",
			expected: "a/b/c/d",
		},
		{
			name:     "yaml file in path unchanged",
			input:    "path/to/schema.yaml",
			expected: "path/to/schema.yaml",
		},
		{
			name:     "json file in path unchanged",
			input:    "path/to/schema.json",
			expected: "path/to/schema.json",
		},
		{
			name:     "yml file in path unchanged",
			input:    "path/to/schema.yml",
			expected: "path/to/schema.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandRegistryShorthand(tt.input)
			if result != tt.expected {
				t.Errorf("expandRegistryShorthand(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
