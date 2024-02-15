package prompts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

const (
	defaultGithubTokenSecretName     = "GITHUB_TOKEN"
	defaultSpeakeasyAPIKeySecretName = "SPEAKEASY_API_KEY"
)

func ConfigureGithub(githubWorkflow *config.GenerateWorkflow, workflow *workflow.Workflow) (*config.GenerateWorkflow, error) {
	if githubWorkflow == nil || githubWorkflow.Jobs.Generate.Uses == "" {
		secrets := make(map[string]string)
		secrets[config.GithubAccessToken] = formatSecretName(defaultGithubTokenSecretName)
		secrets[config.SpeakeasyApiKey] = formatSecretName(defaultSpeakeasyAPIKeySecretName)
		githubWorkflow = &config.GenerateWorkflow{
			Name: "Generate",
			On: config.GenerateOn{
				WorkflowDispatch: config.WorkflowDispatch{
					Inputs: config.Inputs{
						Force: config.Force{
							Description: "Force generation of SDKs",
							Type:        "boolean",
							Default:     false,
						},
					},
				},
				Schedule: []config.Schedule{
					{
						Cron: "0 0 * * *",
					},
				},
			},
			Jobs: config.Jobs{
				Generate: config.Job{
					Uses: "speakeasy-api/sdk-generation-action/.github/workflows/sdk-generation.yaml@v15",
					With: map[string]any{
						"speakeasy_version": "latest",
						"force":             "${{ github.event.inputs.force }}",
						config.Mode:         "pr",
					},
					Secrets: secrets,
				},
			},
			Permissions: config.Permissions{
				Checks:       config.GithubWritePermission,
				Statuses:     config.GithubWritePermission,
				Contents:     config.GithubWritePermission,
				PullRequests: config.GithubWritePermission,
			},
		}
	}

	secrets := githubWorkflow.Jobs.Generate.Secrets
	for _, source := range workflow.Sources {
		for _, input := range source.Inputs {
			if input.Auth != nil {
				secrets[input.Auth.Secret] = formatSecretName(strings.ToLower(input.Auth.Secret))
			}
		}

		for _, overlay := range source.Overlays {
			if overlay.Auth != nil {
				secrets[overlay.Auth.Secret] = formatSecretName(strings.ToLower(overlay.Auth.Secret))
			}
		}
	}
	githubWorkflow.Jobs.Generate.Secrets = secrets
	mode := githubWorkflow.Jobs.Generate.With[config.Mode].(string)

	modeOptions := []huh.Option[string]{
		huh.NewOption(styles.BoldString("pr mode:")+" creates a running PR that you can merge at your convenience [RECOMMENDED]", "pr"),
		huh.NewOption(styles.BoldString("direct mode:")+" attempts to automatically merge changes into your main branch", "direct"),
	}

	prompt := charm.NewSelectPrompt("What mode would you like to setup for your github workflow?\n", "", modeOptions, &mode)
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(prompt),
		"Let's configure generation through github actions.")).
		Run(); err != nil {
		return nil, err
	}
	githubWorkflow.Jobs.Generate.With[config.Mode] = mode

	return githubWorkflow, nil
}

func formatSecretName(name string) string {
	return fmt.Sprintf("${{ secrets.%s }}", name)
}

func FindGithubRepository(outDir string) *git.Repository {
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return nil
	}

	gitFolder, err := filepath.Abs(outDir)
	if err != nil {
		return nil
	}
	prior := ""
	for {
		if _, err := os.Stat(path.Join(gitFolder, ".git")); err == nil {
			break
		}
		prior = gitFolder
		gitFolder = filepath.Dir(gitFolder)
		if gitFolder == prior {
			// No longer have a parent directory
			return nil
		}
	}

	repo, err := git.PlainOpen(gitFolder)
	if err != nil {
		return nil
	}
	return repo
}

func ParseGithubRemoteURL(repo *git.Repository) string {
	cfg, err := repo.ConfigScoped(git_config.SystemScope)
	if err != nil {
		return ""
	}

	var defaultRemote string
	defaultBranch := cfg.Init.DefaultBranch
	if len(defaultBranch) == 0 {
		defaultRemote = git.DefaultRemoteName
	} else {
		defaultBranchConfig, ok := cfg.Branches[defaultBranch]
		if !ok {
			return ""
		}

		defaultRemote = defaultBranchConfig.Remote
	}

	if len(defaultRemote) == 0 {
		return ""
	}

	remoteCfg, ok := cfg.Remotes[defaultRemote]
	if !ok {
		return ""
	}

	for _, url := range remoteCfg.URLs {
		if strings.Contains(url, "https://github.com") {
			return url
		}

		if strings.Contains(url, "git@github.com") {
			return strings.Replace(url, "git@github.com:", "https://github.com/", 1)
		}
	}

	return ""
}
