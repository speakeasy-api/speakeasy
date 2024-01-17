package prompts

import (
	"fmt"

	config "github.com/speakeasy-api/sdk-gen-config"
)

const (
	defaultGithubTokenSecretName     = "GITHUB_TOKEN"
	defaultSpeakeasyAPIKeySecretName = "SPEAKEASY_API_KEY"
)

func githubWorkflowBaseForm(quickstart *Quickstart) (*QuickstartState, error) {
	var targetType, specName string
	for _, target := range quickstart.WorkflowFile.Targets {
		targetType = target.Target
		specName = target.Source
		break
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
					config.Mode:         "pr",
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
