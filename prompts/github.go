package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	git_config "github.com/go-git/go-git/v5/config"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/huh"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm"
	"gopkg.in/yaml.v3"
)

const (
	defaultGithubTokenSecretName     = "GITHUB_TOKEN"
	defaultSpeakeasyAPIKeySecretName = "SPEAKEASY_API_KEY"
	npmTokenDefault                  = "NPM_TOKEN"
	pypiTokenDefault                 = "PYPI_TOKEN"
	nugetTokenDefault                = "NUGET_API_KEY"
	rubyGemsTokenDefault             = "RUBYGEMS_AUTH_TOKEN"
	packagistTokenDefault            = "PACKAGIST_TOKEN"
	ossrhPasswordDefault             = "OSSRH_PASSWORD"
	gpgSecretKeyDefault              = "GPG_SECRET_KEY"
	gpgPassPhraseDefault             = "GPG_PASS_PHRASE"
)

var SupportedPublishingTargets = []string{
	"typescript",
	"python",
	"csharp",
	"ruby",
	"php",
	"java",
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
	return githubWorkflow, nil
}

func ConfigurePublishing(target *workflow.Target, name string) (*workflow.Target, error) {
	promptMap := make(map[publishingPrompt]*string)
	switch target.Target {
	case "typescript":
		currentNpmToken := npmTokenDefault
		if target.Publishing != nil && target.Publishing.NPM != nil {
			currentNpmToken = target.Publishing.NPM.Token
		}
		npmTokenVal := &currentNpmToken
		promptMap[publishingPrompt{
			key:       "NPM Token",
			entryType: publishingTypeSecret,
		}] = npmTokenVal
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
		promptMap[publishingPrompt{
			key:       "PyPI Token",
			entryType: publishingTypeSecret,
		}] = pypiTokenVal
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
		promptMap[publishingPrompt{
			key:       "Nuget API Key",
			entryType: publishingTypeSecret,
		}] = nugetKeyVal
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
		promptMap[publishingPrompt{
			key:       "Ruby Gems Auth Token",
			entryType: publishingTypeSecret,
		}] = rubyGemsTokenVal
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			RubyGems: &workflow.RubyGems{
				Token: formatWorkflowSecret(*rubyGemsTokenVal),
			},
		}
	case "php":
		currentPackagistToken := packagistTokenDefault
		currentPackagistUserName := ""
		if target.Publishing != nil && target.Publishing.Packagist != nil {
			currentPackagistToken = target.Publishing.Packagist.Token
			currentPackagistUserName = target.Publishing.Packagist.Username
		}
		packagistToken := &currentPackagistToken
		packagistUsername := &currentPackagistUserName
		promptMap[publishingPrompt{
			key:       "Packagist Token",
			entryType: publishingTypeSecret,
		}] = packagistToken
		promptMap[publishingPrompt{
			key:       "Packagist Username",
			entryType: publishingTypeValue,
		}] = packagistUsername
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			Packagist: &workflow.Packagist{
				Token:    formatWorkflowSecret(*packagistToken),
				Username: *packagistUsername,
			},
		}
	case "java":
		sonatypeLegacy := target.Publishing != nil && target.Publishing.Java != nil && target.Publishing.Java.UseSonatypeLegacy
		currentGPGSecretKey := gpgSecretKeyDefault
		currentGPGPassPhrase := gpgPassPhraseDefault
		currentossrhPassword := ossrhPasswordDefault
		currentossrhUsername := ""
		if target.Publishing != nil && target.Publishing.Java != nil {
			currentGPGSecretKey = target.Publishing.Java.GPGSecretKey
			currentGPGPassPhrase = target.Publishing.Java.GPGPassPhrase
			currentossrhPassword = target.Publishing.Java.OSSHRPassword
			currentossrhUsername = target.Publishing.Java.OSSRHUsername
		}
		gpgSecretKey := &currentGPGSecretKey
		gpgPassPhrase := &currentGPGPassPhrase
		ossrhPassword := &currentossrhPassword
		ossrhUsername := &currentossrhUsername
		promptMap[publishingPrompt{
			key:       "GPG Secret Key",
			entryType: publishingTypeSecret,
		}] = gpgSecretKey
		promptMap[publishingPrompt{
			key:       "GPG Pass Phrase",
			entryType: publishingTypeSecret,
		}] = gpgPassPhrase
		promptMap[publishingPrompt{
			key:       "OSSRH Password",
			entryType: publishingTypeSecret,
		}] = ossrhPassword
		promptMap[publishingPrompt{
			key:       "OSSRH Username",
			entryType: publishingTypeValue,
		}] = ossrhUsername
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			Java: &workflow.Java{
				GPGSecretKey:      formatWorkflowSecret(*gpgSecretKey),
				GPGPassPhrase:     formatWorkflowSecret(*gpgPassPhrase),
				OSSHRPassword:     formatWorkflowSecret(*ossrhPassword),
				OSSRHUsername:     *ossrhUsername,
				UseSonatypeLegacy: sonatypeLegacy,
			},
		}
	}

	return target, nil
}

type publishingEntryType int

const (
	publishingTypeSecret publishingEntryType = iota
	publishingTypeValue
)

type publishingPrompt struct {
	key       string
	entryType publishingEntryType
}

func executePromptsForPublishing(prompts map[publishingPrompt]*string, target *workflow.Target, name string) error {
	fields := []huh.Field{}
	for prompt, value := range prompts {
		var input *huh.Input
		if prompt.entryType == publishingTypeSecret {
			input = charm.NewInput().
				Title(fmt.Sprintf("Provide a name for your %s secret:", prompt.key)).
				Value(value)
		} else {
			input = charm.NewInput().
				Title(fmt.Sprintf("Provide the value of your %s:", prompt.key)).
				Value(value)
		}
		fields = append(fields,
			input,
		)
	}

	var groups []*huh.Group
	// group two secrets together on a screen
	for i := 0; i < len(fields); i += 2 {
		var groupedFields []huh.Field = []huh.Field{
			fields[i],
		}

		if i+1 < len(fields) {
			groupedFields = append(groupedFields, fields[i+1])
		}
		groups = append(groups, huh.NewGroup(groupedFields...))
	}

	if _, err := charm.NewForm(huh.NewForm(groups...),
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

	if publishing.Packagist != nil {
		secrets = append(secrets, publishing.Packagist.Token)
	}

	if publishing.Java != nil {
		secrets = append(secrets, publishing.Java.GPGSecretKey)
		secrets = append(secrets, publishing.Java.GPGPassPhrase)
		secrets = append(secrets, publishing.Java.OSSHRPassword)
	}

	return secrets
}

func WritePublishing(genWorkflow *config.GenerateWorkflow, workflowFile *workflow.Workflow, workingDir string) (*config.GenerateWorkflow, string, error) {
	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
	secrets[config.SpeakeasyApiKey] = formatGithubSecretName(defaultSpeakeasyAPIKeySecretName)
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
		filePath := filepath.Join(workingDir, ".github/workflows/sdk_publish.yaml")
		publishingFile := &config.PublishWorkflow{}
		if err := readPublishingFile(publishingFile, filePath); err != nil {
			publishingFile = defaultPublishingFile()
		}

		for name, value := range secrets {
			publishingFile.Jobs.Publish.Secrets[name] = value
		}

		// Write a github publishing file.
		var publishingWorkflowBuf bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&publishingWorkflowBuf)
		yamlEncoder.SetIndent(2)
		if err := yamlEncoder.Encode(publishingFile); err != nil {
			return genWorkflow, "", errors.Wrapf(err, "failed to encode workflow file")
		}

		if err := os.WriteFile(filePath, publishingWorkflowBuf.Bytes(), 0o644); err != nil {
			return genWorkflow, filePath, errors.Wrapf(err, "failed to write github publishing file")
		}

		return genWorkflow, filePath, nil
	}

	return genWorkflow, "", nil
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

func defaultPublishingFile() *config.PublishWorkflow {
	return &config.PublishWorkflow{
		Name: "Publish",
		Permissions: config.Permissions{
			Checks:       config.GithubWritePermission,
			Statuses:     config.GithubWritePermission,
			Contents:     config.GithubWritePermission,
			PullRequests: config.GithubWritePermission,
		},
		On: config.PublishOn{
			Push: config.Push{
				Paths: []string{
					"RELEASES.md",
					"*/RELEASES.md",
				},
				Branches: []string{
					"main",
				},
			},
		},
		Jobs: config.Jobs{
			Publish: config.Job{
				Uses:    "speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15",
				Secrets: make(map[string]string),
			},
		},
	}
}

func SelectPublishingTargets(publishingOptions []huh.Option[string], autoSelect bool) ([]string, error) {
	chosenTargets := make([]string, 0)
	if autoSelect {
		for _, option := range publishingOptions {
			chosenTargets = append(chosenTargets, option.Value)
		}
	}
	if _, err := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select targets to configure publishing configs for.").
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
