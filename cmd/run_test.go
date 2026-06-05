package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveParallelTargets(t *testing.T) {
	t.Parallel()

	allTargets := []string{"python-genai", "typescript-genai", "go-genai"}

	tests := []struct {
		name    string
		target  string
		want    []string
		wantErr string
	}{
		{
			name:   "empty runs all targets",
			target: "",
			want:   allTargets,
		},
		{
			name:   "all runs all targets",
			target: "all",
			want:   allTargets,
		},
		{
			name:   "single target",
			target: "python-genai",
			want:   []string{"python-genai"},
		},
		{
			name:   "comma-separated subset",
			target: "python-genai,typescript-genai",
			want:   []string{"python-genai", "typescript-genai"},
		},
		{
			name:   "trims whitespace and skips empties",
			target: " python-genai , , typescript-genai ",
			want:   []string{"python-genai", "typescript-genai"},
		},
		{
			name:    "unknown target errors",
			target:  "python-genai,ruby-genai",
			wantErr: `target "ruby-genai" is not defined in the workflow`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveParallelTargets(tt.target, allTargets)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
