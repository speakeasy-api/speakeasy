package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutionIDComment(t *testing.T) {
	t.Parallel()

	result := ExecutionIDComment("abc123")
	assert.Equal(t, "<!-- execution_id: abc123 -->", result)
}

func TestParseExecutionIDComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard format",
			input:    "<!-- execution_id: abc123 -->",
			expected: "abc123",
		},
		{
			name:     "with extra whitespace",
			input:    "<!--  execution_id:  def456  -->",
			expected: "def456",
		},
		{
			name:     "embedded in text",
			input:    "Some PR body text\n<!-- execution_id: xyz789 -->\nMore text",
			expected: "xyz789",
		},
		{
			name:     "uuid format",
			input:    "<!-- execution_id: 550e8400-e29b-41d4-a716-446655440000 -->",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "not found",
			input:    "Some text without execution id",
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseExecutionIDComment(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutionIDRoundTrip(t *testing.T) {
	t.Parallel()

	// Test that what we serialize can be parsed back
	original := "test-execution-id-12345"
	comment := ExecutionIDComment(original)
	parsed := ParseExecutionIDComment(comment)
	assert.Equal(t, original, parsed)
}
