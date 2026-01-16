package generate

import (
	"bytes"
	"context"
	"testing"

	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersionPairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "single key-value pair",
			input:    "feature,1.0.0",
			expected: map[string]string{"feature": "1.0.0"},
		},
		{
			name:     "multiple key-value pairs",
			input:    "feature1,1.0.0,feature2,2.0.0",
			expected: map[string]string{"feature1": "1.0.0", "feature2": "2.0.0"},
		},
		{
			name:     "odd number of elements - trailing key ignored",
			input:    "feature1,1.0.0,orphan",
			expected: map[string]string{"feature1": "1.0.0"},
		},
		{
			name:     "single element - no pairs formed",
			input:    "orphan",
			expected: map[string]string{},
		},
		{
			name:     "three elements - one pair formed",
			input:    "key1,value1,orphan",
			expected: map[string]string{"key1": "value1"},
		},
		{
			name:     "keys with special characters",
			input:    "go-v2,1.5.0,typescript-v3,2.0.0",
			expected: map[string]string{"go-v2": "1.5.0", "typescript-v3": "2.0.0"},
		},
		{
			name:     "semantic versions",
			input:    "core,1.2.3-beta.1,sdk,0.0.1+build.123",
			expected: map[string]string{"core": "1.2.3-beta.1", "sdk": "0.0.1+build.123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := parseVersionPairs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetChangelogs(t *testing.T) {
	t.Parallel()

	// featureChangelogHeaderPattern matches language-specific changelog feature headers
	// Format: "## featureName: X.Y.Z - YYYY-MM-DD" (e.g., "## core: 3.13.10 - 2026-01-11")
	featureChangelogHeaderPattern := `## \w+: \d+\.\d+\.\d+ - \d{4}-\d{2}-\d{2}`

	tests := []struct {
		name           string
		flags          GenerateSDKChangelogFlags
		expectError    bool
		expectErrorMsg string
		expectPatterns []string // regex patterns to match against output
	}{
		{
			name: "unsupported language returns error",
			flags: GenerateSDKChangelogFlags{
				Language: "unsupported-lang-xyz",
				Raw:      true,
			},
			expectError:    true,
			expectErrorMsg: "unsupported language unsupported-lang-xyz",
		},
		{
			name: "no language with latest version returns changelog",
			flags: GenerateSDKChangelogFlags{
				Raw: true,
			},
			expectError: false,
			// The latest changelog should contain version headers, GitHub links, and commit info
			expectPatterns: []string{
				`## \[v\d+\.\d+\.\d+\]`,
				`github\.com/speakeasy-api/openapi-generation`,
				`\(commit by`,
			},
		},
		{
			name: "go language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "go",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
				`github\.com`,
			},
		},
		{
			name: "typescript language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "typescript",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
			},
		},
		{
			name: "csharp language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "csharp",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
			},
		},
		{
			name: "terraform language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "terraform",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
			},
		},
		{
			name: "php language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "php",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
			},
		},
		{
			name: "ruby language fetches latest versions and returns feature changelogs",
			flags: GenerateSDKChangelogFlags{
				Language: "ruby",
				Raw:      true,
			},
			expectError: false,
			expectPatterns: []string{
				featureChangelogHeaderPattern,
				`\(commit by`,
			},
		},
		{
			name: "odd number of version pairs handled gracefully without panic",
			flags: GenerateSDKChangelogFlags{
				Language:      "go",
				TargetVersion: "core,1.0.0,orphan",
				Raw:           true,
			},
			expectError: false,
			// Should still work with partial pairs - no expected content since version may not exist
			expectPatterns: []string{},
		},
		{
			name: "single element version string handled gracefully without panic",
			flags: GenerateSDKChangelogFlags{
				Language:      "go",
				TargetVersion: "orphan",
				Raw:           true,
			},
			expectError: false,
			// Empty map results in no changelog, but no error
			expectPatterns: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := log.New().WithWriter(&buf)
			ctx := log.With(context.Background(), logger)

			err := getChangelogs(ctx, tt.flags)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectErrorMsg)
				}
				return
			}

			require.NoError(t, err)

			output := buf.String()

			for _, pattern := range tt.expectPatterns {
				assert.Regexp(t, pattern, output)
			}
		})
	}
}

func TestGetChangelogs_RawFalseWithNonInteractive(t *testing.T) {
	t.Parallel()

	// When Raw=false but the terminal is not interactive (which is the case in tests),
	// the code path still uses raw output due to: raw := flags.Raw || !utils.IsInteractive()
	// This test verifies that behavior - the output should still be valid changelog content
	var buf bytes.Buffer
	logger := log.New().WithWriter(&buf)
	ctx := log.With(context.Background(), logger)

	err := getChangelogs(ctx, GenerateSDKChangelogFlags{
		Raw: false,
	})

	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output, "expected non-empty output")
	// The output should contain changelog content (version headers like "## [v2.x.x]")
	assert.Contains(t, output, "## [v", "expected output to contain version headers")
}
