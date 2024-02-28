package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	"github.com/pkg/errors"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"gopkg.in/yaml.v3"
)

const (
	defaultGithubTokenSecretName     = "GITHUB_TOKEN"
	defaultSpeakeasyAPIKeySecretName = "SPEAKEASY_API_KEY"
	npmTokenDefault                  = "NPM_TOKEN"
	pypiTokenDefault                 = "PYPI_TOKEN"
	nugetTokenDefault                = "NUGET_API_KEY"
	rubyGemsTokenDefault             = "RUBYGEMS_AUTH_TOKEN"
)

var SupportedPublishingTargets = []string{
	"typescript",
	"python",
	"csharp",
	"ruby",
}

func ConfigureGithub(githubWorkflow *config.GenerateWorkflow, workflow *workflow.Workflow) (*config.GenerateWorkflow, error) {
	if githubWorkflow == nil || githubWorkflow.Jobs.Generate.Uses == "" {
		githubWorkflow = defaultGenerationFile()
	}

	secrets := githubWorkflow.Jobs.Generate.Secrets
	for _, source := range workflow.Sources {
		for _, input := range source.Inputs {
			if input.Auth != nil {
				secrets[formatGithubSecret(input.Auth.Secret)] = formatGithubSecretName(input.Auth.Secret)
			}
		}

		for _, overlay := range source.Overlays {
			if overlay.Auth != nil {
				secrets[formatGithubSecret(overlay.Auth.Secret)] = formatGithubSecretName(overlay.Auth.Secret)
			}
		}
	}
	githubWorkflow.Jobs.Generate.Secrets = secrets
	mode := githubWorkflow.Jobs.Generate.With[config.Mode].(string)

	modeOptions := []huh.Option[string]{
		huh.NewOption(styles.MakeBold("pr mode:")+" creates a running PR that you can merge at your convenience [RECOMMENDED]", "pr"),
		huh.NewOption(styles.MakeBold("direct mode:")+" attempts to automatically merge changes into your main branch", "direct"),
	}

	prompt := charm.NewSelectPrompt("What mode would you like to setup for your github workflow?\n", "", modeOptions, &mode)
	if _, err := charm.NewForm(huh.NewForm(prompt),
		"Let's configure generation through github actions.").
		ExecuteForm(); err != nil {
		return nil, err
	}
	githubWorkflow.Jobs.Generate.With[config.Mode] = mode

	return githubWorkflow, nil
}

func ConfigurePublishing(target *workflow.Target, name string) (*workflow.Target, error) {
	promptMap := make(map[string]*string)
	switch target.Target {
	case "typescript":
		currentNpmToken := npmTokenDefault
		if target.Publishing != nil && target.Publishing.NPM != nil {
			currentNpmToken = target.Publishing.NPM.Token
		}
		npmTokenVal := &currentNpmToken
		promptMap["NPM Token"] = npmTokenVal
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			NPM: &workflow.NPM{
				Token: formatWorkflowSecret(*npmTokenVal),
			},
		}
	case "python":
		currentPyPIToken := pypiTokenDefault
		if target.Publishing != nil && target.Publishing.PyPi != nil {
			currentPyPIToken = target.Publishing.PyPi.Token
		}
		pypiTokenVal := &currentPyPIToken
		promptMap["PyPI Token"] = pypiTokenVal
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			PyPi: &workflow.PyPi{
				Token: formatWorkflowSecret(*pypiTokenVal),
			},
		}
	case "csharp":
		currentNugetKey := nugetTokenDefault
		if target.Publishing != nil && target.Publishing.Nuget != nil {
			currentNugetKey = target.Publishing.Nuget.APIKey
		}
		nugetKeyVal := &currentNugetKey
		promptMap["Nuget API Key"] = nugetKeyVal
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			Nuget: &workflow.Nuget{
				APIKey: formatWorkflowSecret(*nugetKeyVal),
			},
		}
	case "ruby":
		currentRubyGemsToken := rubyGemsTokenDefault
		if target.Publishing != nil && target.Publishing.RubyGems != nil {
			currentRubyGemsToken = target.Publishing.RubyGems.Token
		}
		rubyGemsTokenVal := &currentRubyGemsToken
		promptMap["Ruby Gems Auth Token"] = rubyGemsTokenVal
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			RubyGems: &workflow.RubyGems{
				Token: formatWorkflowSecret(*rubyGemsTokenVal),
			},
		}
	}

	return target, nil
}

func executePromptsForPublishing(prompts map[string]*string, target *workflow.Target, name string) error {
	fields := []huh.Field{}
	for prompt, value := range prompts {
		fields = append(fields,
			charm.NewInput().
				Title(fmt.Sprintf("Provide a value for %s:", prompt)).
				Value(value),
		)
	}

	if _, err := charm.NewForm(huh.NewForm(huh.NewGroup(fields...)),
		fmt.Sprintf("Setup publishing variables for your %s target %s.", target.Target, name),
		"These environment variables will be used to publish to package managers from your speakeasy workflow.").
		ExecuteForm(); err != nil {
		return err
	}

	return nil
}

func formatGithubSecretName(name string) string {
	return fmt.Sprintf("${{ secrets.%s }}", strings.ToUpper(formatGithubSecret(name)))
}

func formatWorkflowSecret(secret string) string {
	if secret != "" && secret[0] != '$' {
		secret = "$" + secret
	}
	return strings.ToLower(secret)
}

func formatGithubSecret(secret string) string {
	if secret != "" && secret[0] == '$' {
		secret = secret[1:]
	}
	return strings.ToLower(secret)
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
		if strings.Contains(url, "git@github.com") {
			url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
		}

		if strings.HasSuffix(url, ".git") {
			url = url[:len(url)-4]
		}

		return url
	}

	return ""
}

func getSecretsValuesFromPublishing(publishing workflow.Publishing) []string {
	secrets := []string{}

	if publishing.PyPi != nil {
		secrets = append(secrets, publishing.PyPi.Token)
	}

	if publishing.NPM != nil {
		secrets = append(secrets, publishing.NPM.Token)
	}

	if publishing.RubyGems != nil {
		secrets = append(secrets, publishing.RubyGems.Token)
	}

	if publishing.Nuget != nil {
		secrets = append(secrets, publishing.Nuget.APIKey)
	}

	return secrets
}

func WritePublishing(genWorkflow *config.GenerateWorkflow, workflowFile *workflow.Workflow, workingDir string) (*config.GenerateWorkflow, error) {
	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
	for _, target := range workflowFile.Targets {
		if target.Publishing != nil {
			for _, secret := range getSecretsValuesFromPublishing(*target.Publishing) {
				secrets[formatGithubSecret(secret)] = formatGithubSecretName(secret)
			}
		}
	}

	currentSecrets := genWorkflow.Jobs.Generate.Secrets
	for secret, value := range secrets {
		currentSecrets[secret] = value
	}
	genWorkflow.Jobs.Generate.Secrets = currentSecrets

	mode := genWorkflow.Jobs.Generate.With[config.Mode].(string)
	if mode == "pr" {
		publishingTargets := make(map[string]workflow.Target)
		publishingFilePaths := make(map[string]string)
		for name, target := range workflowFile.Targets {
			if target.Publishing != nil {
				publishingTargets[name] = target
			}
		}

		for name := range publishingTargets {
			if len(publishingTargets) == 1 {
				publishingFilePaths[name] = filepath.Join(workingDir, ".github/workflows/sdk_publish.yaml")
			} else {
				publishingFilePaths[name] = filepath.Join(workingDir, fmt.Sprintf(".github/workflows/%s/sdk_publish.yaml", name))
			}
		}

		for name, target := range publishingTargets {
			filePath := publishingFilePaths[name]
			publishingFile := &config.PublishWorkflow{}
			if err := readPublishingFile(publishingFile, filePath); err != nil {
				actionName := "Publish"
				if len(publishingTargets) > 1 {
					actionName += " " + name
				}
				publishingFile = defaultPublishingFile(actionName)
			}

			if target.Output != nil {
				publishingFile.On.Push.Paths = []string{fmt.Sprintf("%s/RELEASES.md", *target.Output)}
			}

			publishingFile.Jobs.Publish.With[fmt.Sprintf("publish_%s", target.Target)] = true
			publishingFile.Jobs.Publish.Secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
			for _, secret := range getSecretsValuesFromPublishing(*target.Publishing) {
				publishingFile.Jobs.Publish.Secrets[formatGithubSecret(secret)] = formatGithubSecretName(secret)
			}

			// Write a github publishing file.
			var publishingWorkflowBuf bytes.Buffer
			yamlEncoder := yaml.NewEncoder(&publishingWorkflowBuf)
			yamlEncoder.SetIndent(2)
			if err := yamlEncoder.Encode(publishingFile); err != nil {
				return genWorkflow, errors.Wrapf(err, "failed to encode workflow file")
			}

			if _, err := os.Stat(strings.Replace(filePath, "/sdk_publish.yaml", "", -1)); os.IsNotExist(err) {
				err = os.MkdirAll(strings.Replace(filePath, "/sdk_publish.yaml", "", -1), 0o755)
				if err != nil {
					return genWorkflow, err
				}
			}

			if err := os.WriteFile(filePath, publishingWorkflowBuf.Bytes(), 0o644); err != nil {
				return genWorkflow, errors.Wrapf(err, "failed to write github publishing file")
			}
		}

	}

	return genWorkflow, nil
}

func WriteGenerationFile(generationWorkflow *config.GenerateWorkflow, generationWorkflowFilePath string) error {
	var genWorkflowBuf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&genWorkflowBuf)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(generationWorkflow); err != nil {
		return errors.Wrapf(err, "failed to encode workflow file")
	}

	if err := os.WriteFile(generationWorkflowFilePath, genWorkflowBuf.Bytes(), 0o644); err != nil {
		return errors.Wrapf(err, "failed to write github workflow file")
	}

	return nil
}

func ReadGenerationFile(generationWorkflow *config.GenerateWorkflow, generationWorkflowFilePath string) error {
	if _, err := os.Stat(generationWorkflowFilePath); err != nil {
		return err
	}

	fileContent, err := os.ReadFile(generationWorkflowFilePath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(fileContent, generationWorkflow); err != nil {
		return err
	}

	return nil
}

func readPublishingFile(publishingFile *config.PublishWorkflow, publishingWorkflowFilePath string) error {
	if _, err := os.Stat(publishingWorkflowFilePath); err != nil {
		return err
	}

	fileContent, err := os.ReadFile(publishingWorkflowFilePath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(fileContent, publishingFile); err != nil {
		return err
	}

	return nil
}

func defaultGenerationFile() *config.GenerateWorkflow {
	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
	secrets[config.SpeakeasyApiKey] = formatGithubSecretName(defaultSpeakeasyAPIKeySecretName)
	return &config.GenerateWorkflow{
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
				Uses: "speakeasy-api/sdk-generation-action/.github/workflows/workflow-executor.yaml@v15",
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

func defaultPublishingFile(name string) *config.PublishWorkflow {
	return &config.PublishWorkflow{
		Name: name,
		On: config.PublishOn{
			Push: config.Push{
				Paths: []string{
					"RELEASES.md",
				},
				Branches: []string{
					"main",
				},
			},
		},
		Jobs: config.Jobs{
			Publish: config.Job{
				Uses: "speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15",
				With: map[string]any{
					"create_release": true,
				},
				Secrets: make(map[string]string),
			},
		},
	}
}

func SelectPublishingTargets(publishingOptions []huh.Option[string]) ([]string, error) {
	chosenTargets := make([]string, 0)
	if _, err := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select any targets you would like to configure publishing for.").
			Description("Setup variables to configure publishing directly from Speakeasy.\n").
			Options(publishingOptions...).
			Value(&chosenTargets),
	)),
		"Would you like to configure publishing for any existing targets?").
		ExecuteForm(); err != nil {
		return nil, err
	}

	return chosenTargets, nil
}
