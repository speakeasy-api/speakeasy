package prompts_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	config "github.com/speakeasy-api/sdk-gen-config"
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

func TestConfigurePublishingCSharpTrustedPublishing(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "csharp",
		Source: "api",
	}

	updated, err := prompts.ConfigurePublishing(target, "csharp", prompts.ConfigurePublishingOptions{NuGetTrustedPublishing: true})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing)
	require.NotNil(t, updated.Publishing.Nuget)
	assert.Empty(t, updated.Publishing.Nuget.APIKey)
	assert.Equal(t, "$nuget_username", updated.Publishing.Nuget.Username)
	require.NotNil(t, updated.Publishing.Nuget.UseTrustedPublishing)
	assert.True(t, *updated.Publishing.Nuget.UseTrustedPublishing)
}

func TestConfigurePublishingCSharpTrustedPublishingReplacesAPIKeyConfig(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "csharp",
		Source: "api",
		Publishing: &workflow.Publishing{
			Nuget: &workflow.Nuget{
				APIKey: "$nuget_api_key",
			},
		},
	}

	updated, err := prompts.ConfigurePublishing(target, "csharp", prompts.ConfigurePublishingOptions{NuGetTrustedPublishing: true})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing.Nuget)
	assert.Empty(t, updated.Publishing.Nuget.APIKey)
	assert.Equal(t, "$nuget_username", updated.Publishing.Nuget.Username)
	require.NotNil(t, updated.Publishing.Nuget.UseTrustedPublishing)
	assert.True(t, *updated.Publishing.Nuget.UseTrustedPublishing)
}

func TestWritePublishingCSharpTrustedPublishingAddsCallerJob(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	existingPublishing := `name: Publish
permissions:
  checks: write
  contents: write
  pull-requests: write
  statuses: write
"on":
  push:
    branches:
      - main
    paths:
      - sdk/.speakeasy/gen.lock
jobs:
  publish:
    uses: speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15
    with:
      target: csharp
    secrets:
      github_access_token: ${{ secrets.GITHUB_TOKEN }}
      nuget_api_key: ${{ secrets.NUGET_API_KEY }}
      speakeasy_api_key: ${{ secrets.SPEAKEASY_API_KEY }}
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "sdk_publish.yaml"), []byte(existingPublishing), 0o644))
	useTrustedPublishing := true
	target := workflow.Target{
		Target: "csharp",
		Source: "api",
		Publishing: &workflow.Publishing{
			Nuget: &workflow.Nuget{
				Username:             "$nuget_username",
				UseTrustedPublishing: &useTrustedPublishing,
			},
		},
	}
	wf := &workflow.Workflow{Targets: map[string]workflow.Target{"csharp": target}}
	genWorkflow := &config.GenerateWorkflow{
		Jobs: config.Jobs{
			Generate: config.Job{
				With:    map[string]any{config.Mode: "pr"},
				Secrets: map[string]string{},
			},
		},
	}

	paths, err := prompts.WritePublishing(wf, genWorkflow, "csharp", dir, "", target)
	require.NoError(t, err)
	require.Len(t, paths, 1)

	contents, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	publishing := string(contents)

	assert.Contains(t, publishing, "id-token: write")
	assert.Contains(t, publishing, "nuget_username: ${{ secrets.NUGET_USERNAME }}")
	assert.NotContains(t, publishing, "nuget_api_key")
	assert.Contains(t, publishing, "publish-nuget:")
	assert.Contains(t, publishing, "needs.publish.outputs.use_nuget_trusted_publishing == 'true'")
	assert.Contains(t, publishing, "uses: speakeasy-api/sdk-generation-action/publish-nuget@v15")
	assert.Contains(t, publishing, "csharp-directory: ${{ needs.publish.outputs.csharp_directory }}")
	assert.Contains(t, publishing, "nuget-username: ${{ secrets.NUGET_USERNAME }}")
}

func TestConfigurePublishingRubyTrustedPublishing(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "ruby",
		Source: "api",
	}

	updated, err := prompts.ConfigurePublishing(target, "ruby", prompts.ConfigurePublishingOptions{RubyGemsTrustedPublishing: true})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing)
	require.NotNil(t, updated.Publishing.RubyGems)
	assert.Empty(t, updated.Publishing.RubyGems.Token)
	assert.Equal(t, "$rubygems_owner", updated.Publishing.RubyGems.Owner)
	require.NotNil(t, updated.Publishing.RubyGems.UseTrustedPublishing)
	assert.True(t, *updated.Publishing.RubyGems.UseTrustedPublishing)
}

func TestConfigurePublishingRubyTrustedPublishingReplacesTokenConfig(t *testing.T) {
	t.Parallel()

	target := &workflow.Target{
		Target: "ruby",
		Source: "api",
		Publishing: &workflow.Publishing{
			RubyGems: &workflow.RubyGems{
				Token: "$rubygems_auth_token",
			},
		},
	}

	updated, err := prompts.ConfigurePublishing(target, "ruby", prompts.ConfigurePublishingOptions{RubyGemsTrustedPublishing: true})
	require.NoError(t, err)
	require.NotNil(t, updated.Publishing.RubyGems)
	assert.Empty(t, updated.Publishing.RubyGems.Token)
	require.NotNil(t, updated.Publishing.RubyGems.UseTrustedPublishing)
	assert.True(t, *updated.Publishing.RubyGems.UseTrustedPublishing)
}

func TestWritePublishingRubyTrustedPublishingAddsCallerJob(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	existingPublishing := `name: Publish
permissions:
  checks: write
  contents: write
  pull-requests: write
  statuses: write
"on":
  push:
    branches:
      - main
    paths:
      - sdk/.speakeasy/gen.lock
jobs:
  publish:
    uses: speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15
    with:
      target: ruby
    secrets:
      github_access_token: ${{ secrets.GITHUB_TOKEN }}
      rubygems_auth_token: ${{ secrets.RUBYGEMS_AUTH_TOKEN }}
      speakeasy_api_key: ${{ secrets.SPEAKEASY_API_KEY }}
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "sdk_publish.yaml"), []byte(existingPublishing), 0o644))
	useTrustedPublishing := true
	target := workflow.Target{
		Target: "ruby",
		Source: "api",
		Publishing: &workflow.Publishing{
			RubyGems: &workflow.RubyGems{
				Owner:                "$rubygems_owner",
				UseTrustedPublishing: &useTrustedPublishing,
			},
		},
	}
	wf := &workflow.Workflow{Targets: map[string]workflow.Target{"ruby": target}}
	genWorkflow := &config.GenerateWorkflow{
		Jobs: config.Jobs{
			Generate: config.Job{
				With:    map[string]any{config.Mode: "pr"},
				Secrets: map[string]string{"rubygems_auth_token": "${{ secrets.RUBYGEMS_AUTH_TOKEN }}"},
			},
		},
	}

	paths, err := prompts.WritePublishing(wf, genWorkflow, "ruby", dir, "", target)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	assert.NotContains(t, genWorkflow.Jobs.Generate.Secrets, "rubygems_auth_token")
	assert.Equal(t, "${{ secrets.RUBYGEMS_OWNER }}", genWorkflow.Jobs.Generate.Secrets["rubygems_owner"])

	contents, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	publishing := string(contents)

	assert.Contains(t, publishing, "id-token: write")
	assert.NotContains(t, publishing, "rubygems_auth_token")
	assert.Contains(t, publishing, "publish-rubygems:")
	assert.Contains(t, publishing, "needs.publish.outputs.use_rubygems_trusted_publishing == 'true'")
	assert.Contains(t, publishing, "uses: speakeasy-api/sdk-generation-action/publish-rubygems@v15")
	assert.Contains(t, publishing, "ruby-directory: ${{ needs.publish.outputs.ruby_directory }}")
	assert.Contains(t, publishing, "rubygems_owner: ${{ secrets.RUBYGEMS_OWNER }}")
	assert.Contains(t, publishing, "rubygems-owner: ${{ secrets.RUBYGEMS_OWNER }}")
}
