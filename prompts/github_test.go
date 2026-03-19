package prompts_test

import (
	"testing"

	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/prompts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurePublishingCLI(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "cli",
		Source: "api",
	}

	updated, err := prompts.ConfigurePublishing(target, "cli", prompts.ConfigurePublishingOptions{})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing)
	require.NotNil(t, updated.Publishing.CLI)
	assert.Equal(t, "$cli_gpg_private_key", updated.Publishing.CLI.GPGPrivateKey)
	assert.Equal(t, "$cli_gpg_passphrase", updated.Publishing.CLI.GPGPassPhrase)
}

func TestConfigurePublishingCLIPreservesExistingConfig(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "cli",
		Source: "api",
		Publishing: &workflow.Publishing{
			CLI: &workflow.CLI{
				GPGPrivateKey: "$existing_key",
				GPGPassPhrase: "$existing_passphrase",
			},
		},
	}

	updated, err := prompts.ConfigurePublishing(target, "cli", prompts.ConfigurePublishingOptions{})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing)
	require.NotNil(t, updated.Publishing.CLI)
	assert.Equal(t, "$existing_key", updated.Publishing.CLI.GPGPrivateKey)
	assert.Equal(t, "$existing_passphrase", updated.Publishing.CLI.GPGPassPhrase)
}

func TestParseGithubRemoteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		remoteURL   string
		expectedURL string
	}{
		{
			name:        "SSH URL with .git suffix",
			remoteURL:   "git@github.com:speakeasy-api/test-repo.git",
			expectedURL: "https://github.com/speakeasy-api/test-repo",
		},
		{
			name:        "SSH URL without .git suffix",
			remoteURL:   "git@github.com:speakeasy-api/test-repo",
			expectedURL: "https://github.com/speakeasy-api/test-repo",
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
			remoteURL:   "git@github.com:Speakeasy-API/test-repo.git",
			expectedURL: "https://github.com/Speakeasy-API/test-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a temporary directory for the git repo
			tmpDir := t.TempDir()

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
