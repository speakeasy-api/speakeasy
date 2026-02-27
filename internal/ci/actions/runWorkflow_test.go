package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldDeleteBranch(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		branchName     string
		featureBranch  string
		debug          string
		success        bool
		expectedDelete bool
	}{
		{
			name:           "direct mode success — delete",
			mode:           "direct",
			success:        true,
			expectedDelete: true,
		},
		{
			name:           "direct mode failure — delete",
			mode:           "direct",
			success:        false,
			expectedDelete: true,
		},
		{
			name:           "pr mode success — keep",
			mode:           "pr",
			success:        true,
			expectedDelete: false,
		},
		{
			name:           "pr mode failure — delete",
			mode:           "pr",
			success:        false,
			expectedDelete: true,
		},
		{
			name:           "explicit branch name — never delete",
			mode:           "direct",
			branchName:     "speakeasy-sdk-regen",
			success:        true,
			expectedDelete: false,
		},
		{
			name:           "explicit branch name on failure — still never delete",
			mode:           "direct",
			branchName:     "speakeasy-sdk-regen",
			success:        false,
			expectedDelete: false,
		},
		{
			name:           "feature branch — never delete",
			mode:           "direct",
			featureBranch:  "feature/my-branch",
			success:        true,
			expectedDelete: false,
		},
		{
			name:           "debug mode — never delete",
			mode:           "direct",
			debug:          "true",
			success:        true,
			expectedDelete: false,
		},
		{
			name:           "matrix mode — never delete (implicit via branch name)",
			mode:           "matrix",
			branchName:     "speakeasy-sdk-regen",
			success:        true,
			expectedDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("INPUT_MODE", tt.mode)
			t.Setenv("INPUT_BRANCH_NAME", tt.branchName)
			t.Setenv("INPUT_FEATURE_BRANCH", tt.featureBranch)
			t.Setenv("INPUT_DEBUG", tt.debug)
			t.Setenv("SPEAKEASY_TEST_MODE", "")

			result := shouldDeleteBranch(tt.success)
			assert.Equal(t, tt.expectedDelete, result)
		})
	}
}
