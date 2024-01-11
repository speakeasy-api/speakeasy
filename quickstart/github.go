package quickstart

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/speakeasy/charm"
)

const (
	defaultGithubTokenSecretName     = "GITHUB_TOKEN"
	defaultSpeakeasyAPIKeySecretName = "SPEAKEASY_API_KEY"
)

func githubWorkflowBaseForm(quickstart *Quickstart) (*State, error) {
	var targetType, targetName, specName string
	for key, target := range quickstart.WorkflowFile.Targets {
		targetName = key
		targetType = target.Target
		specName = target.Source
		break
	}

	var workflowMode string
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How should changes be integrated into your main branch?").
				Description("`pr` mode is recommended. This will auto-update a PR with generation target changes. \n"+
					"`direct` mode will automatically merge any generation target changes into main for you.").
				Options(huh.NewOptions([]string{"pr", "direct"}...)...).
				Value(&workflowMode),
		)),
		fmt.Sprintf("Let's setup a github workflow file for generating your %s target (%s)", targetType, targetName),
		"Generating your target through Github Actions is a very useful way to ensure that your SDK stays up to date.")).
		Run(); err != nil {
		return nil, err
	}

	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatSecretName(defaultGithubTokenSecretName)
	secrets[config.SpeakeasyApiKey] = formatSecretName(defaultSpeakeasyAPIKeySecretName)

	genWorkflow := &config.GenerateWorkflow{
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
				Uses: "speakeasy-api/sdk-generation-action/.github/workflows/sdk-generation.yaml@v14",
				With: map[string]any{
					"speakeasy_version": "latest",
					"force":             "${{ github.event.inputs.force }}",
					config.OpenAPIDocs:  fmt.Sprintf("- %s\n", quickstart.WorkflowFile.Sources[specName].Inputs[0].Location),
					config.Mode:         workflowMode,
					config.Languages:    fmt.Sprintf("- %s\n", targetType),
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

	if quickstart.WorkflowFile.Sources[specName].Inputs[0].Auth != nil {
		genWorkflow.Jobs.Generate.With[config.OpenAPIDocAuthHeader] = quickstart.WorkflowFile.Sources[specName].Inputs[0].Auth.Header
		genWorkflow.Jobs.Generate.Secrets[config.OpenAPIDocAuthToken] = formatSecretName(quickstart.WorkflowFile.Sources[specName].Inputs[0].Auth.Secret)
	}

	quickstart.GithubWorkflow = genWorkflow
	nextState := Complete
	return &nextState, nil
}

func formatSecretName(name string) string {
	return fmt.Sprintf("${{ secrets.%s }}", name)
}
