package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMode(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected Mode
	}{
		{
			name:     "empty defaults to direct",
			envValue: "",
			expected: ModeDirect,
		},
		{
			name:     "direct",
			envValue: "direct",
			expected: ModeDirect,
		},
		{
			name:     "pr",
			envValue: "pr",
			expected: ModePR,
		},
		{
			name:     "matrix",
			envValue: "matrix",
			expected: ModeMatrix,
		},
		{
			name:     "test",
			envValue: "test",
			expected: ModeTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("INPUT_MODE", tt.envValue)
			assert.Equal(t, tt.expected, GetMode())
		})
	}
}
