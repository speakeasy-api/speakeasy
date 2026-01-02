package prompts_test

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGithubRemoteURL(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		expectedURL string
	}{
		{
			name:        "SSH URL with .git suffix",
			remoteURL:   "git@github.com:quiverai/quiverai-node.git",
			expectedURL: "https://github.com/quiverai/quiverai-node",
		},
		{
			name:        "SSH URL without .git suffix",
			remoteURL:   "git@github.com:quiverai/quiverai-node",
			expectedURL: "https://github.com/quiverai/quiverai-node",
		},
		{
			name:        "HTTPS URL with .git suffix",
			remoteURL:   "https://github.com/speakeasy-api/speakeasy.git",
			expectedURL: "https://github.com/speakeasy-api/speakeasy",
		},
		{
			name:        "HTTPS URL without .git suffix",
			remoteURL:   "https://github.com/speakeasy-api/speakeasy",
			expectedURL: "https://github.com/speakeasy-api/speakeasy",
		},
		{
			name:        "SSH URL with mixed case org",
			remoteURL:   "git@github.com:QuiverAI/quiverai-node.git",
			expectedURL: "https://github.com/QuiverAI/quiverai-node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory for the git repo
			tmpDir, err := os.MkdirTemp("", "github-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Initialize a git repository
			repo, err := git.PlainInit(tmpDir, false)
			require.NoError(t, err)

			// Configure the remote
			cfg, err := repo.Config()
			require.NoError(t, err)

			cfg.Remotes["origin"] = &git_config.RemoteConfig{
				Name: "origin",
				URLs: []string{tt.remoteURL},
			}

			// Set the default branch to point to origin
			cfg.Branches["main"] = &git_config.Branch{
				Name:   "main",
				Remote: "origin",
			}
			cfg.Init.DefaultBranch = "main"

			err = repo.SetConfig(cfg)
			require.NoError(t, err)

			// Call the function under test
			result := prompts.ParseGithubRemoteURL(repo)

			assert.Equal(t, tt.expectedURL, result)
		})
	}
}
