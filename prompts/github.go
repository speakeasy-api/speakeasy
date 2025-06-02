package prompts

import (
	"bytes"
	"context"
	_ "embed"
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
	"github.com/speakeasy-api/speakeasy/internal/log"
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
	packagistUsernameDefault         = "PACKAGIST_USERNAME"
	ossrhPasswordDefault             = "OSSRH_PASSWORD"
	osshrUsernameDefault             = "OSSRH_USERNAME"
	gpgSecretKeyDefault              = "JAVA_GPG_SECRET_KEY"
	gpgPassPhraseDefault             = "JAVA_GPG_PASSPHRASE"
	terraformGPGPrivateKeyDefault    = "TERRAFORM_GPG_PRIVATE_KEY"
	terraformGPGPassPhraseDefault    = "TERRAFORM_GPG_PASSPHRASE"
)

var SupportedPublishingTargets = []string{
	"csharp",
	"go",
	"java",
	"mcp-typescript",
	"php",
	"python",
	"ruby",
	"terraform",
	"typescript",
}

var SupportedTestingTargets = []string{
	"go",
	"java",
	"python",
	"typescript",
}

//go:embed terraform_release.yaml
var terraformReleaseAction string

//go:embed terraform_releaser.yaml
var goReleaser string

func ConfigureGithub(githubWorkflow *config.GenerateWorkflow, workflow *workflow.Workflow, workflowFileDir string, target *string) (*config.GenerateWorkflow, error) {
	if githubWorkflow == nil || githubWorkflow.Jobs.Generate.Uses == "" {
		githubWorkflow = defaultGenerationFile()
	}

	// backfill id-token write permissions
	if githubWorkflow.Permissions.IDToken != config.GithubWritePermission {
		githubWorkflow.Permissions.IDToken = config.GithubWritePermission
	}

	if target != nil {
		githubWorkflow.Name = fmt.Sprintf("Generate %s", strings.ToUpper(*target))
		githubWorkflow.Jobs.Generate.With["target"] = *target
	}

	if target == nil && len(workflow.Targets) > 1 {
		githubWorkflow.On.WorkflowDispatch.Inputs.Target = &config.Target{
			Description: "optionally: set a specific target to generate, default is all",
			Type:        "string",
		}
		githubWorkflow.Jobs.Generate.With["target"] = "${{ github.event.inputs.target }}"
	}

	githubWorkflow.On.WorkflowDispatch.Inputs.SetVersion = &config.SetVersion{
		Description: "optionally set a specific SDK version",
		Type:        "string",
	}
	githubWorkflow.Jobs.Generate.With["set_version"] = "${{ github.event.inputs.set_version }}"
	if workflowFileDir != "" {
		githubWorkflow.Jobs.Generate.With["working_directory"] = workflowFileDir
	}

	secrets := githubWorkflow.Jobs.Generate.Secrets
	for _, source := range workflow.Sources {
		for _, input := range source.Inputs {
			if input.Auth != nil {
				secrets[formatGithubSecret(input.Auth.Secret)] = formatGithubSecretName(input.Auth.Secret)
			}
		}

		for _, overlay := range source.Overlays {
			if overlay.Document != nil && overlay.Document.Auth != nil {
				secrets[formatGithubSecret(overlay.Document.Auth.Secret)] = formatGithubSecretName(overlay.Document.Auth.Secret)
			}
		}
	}
	githubWorkflow.Jobs.Generate.Secrets = secrets
	return githubWorkflow, nil
}

func ConfigurePublishing(target *workflow.Target, name string) (*workflow.Target, error) {
	promptMap := make(map[publishingPrompt]*string)

	// If the target already has a publishing definition for the package manager
	// for the target language, return the target unmodified.
	hasPublishingDefined := target.Publishing != nil

	if hasPublishingDefined {
		switch target.Target {
		case "mcp-typescript", "typescript":
			if target.Publishing.NPM != nil {
				return target, nil
			}
		case "python":
			if target.Publishing.PyPi != nil {
				return target, nil
			}
		case "csharp":
			if target.Publishing.Nuget != nil {
				return target, nil
			}
		case "ruby":
			if target.Publishing.RubyGems != nil {
				return target, nil
			}
		case "php":
			if target.Publishing.Packagist != nil {
				return target, nil
			}
		case "java":
			if target.Publishing.Java != nil {
				return target, nil
			}
		case "terraform":
			if target.Publishing.Terraform != nil {
				return target, nil
			}
		}
	}

	switch target.Target {
	case "mcp-typescript", "typescript":
		target.Publishing = &workflow.Publishing{
			NPM: &workflow.NPM{
				Token: formatWorkflowSecret(npmTokenDefault),
			},
		}
	case "python":
		target.Publishing = &workflow.Publishing{
			PyPi: &workflow.PyPi{
				Token: formatWorkflowSecret(pypiTokenDefault),
			},
		}
	case "csharp":
		target.Publishing = &workflow.Publishing{
			Nuget: &workflow.Nuget{
				APIKey: formatWorkflowSecret(nugetTokenDefault),
			},
		}
	case "ruby":
		target.Publishing = &workflow.Publishing{
			RubyGems: &workflow.RubyGems{
				Token: formatWorkflowSecret(rubyGemsTokenDefault),
			},
		}
	case "php":
		target.Publishing = &workflow.Publishing{
			Packagist: &workflow.Packagist{
				Token:    formatWorkflowSecret(packagistTokenDefault),
				Username: formatWorkflowSecret(packagistUsernameDefault),
			},
		}
	case "java":
		sonatypeLegacy := target.Publishing != nil && target.Publishing.Java != nil && target.Publishing.Java.UseSonatypeLegacy
		if err := executePromptsForPublishing(promptMap, target, name); err != nil {
			return nil, err
		}
		target.Publishing = &workflow.Publishing{
			Java: &workflow.Java{
				GPGSecretKey:      formatWorkflowSecret(gpgSecretKeyDefault),
				GPGPassPhrase:     formatWorkflowSecret(gpgPassPhraseDefault),
				OSSHRPassword:     formatWorkflowSecret(ossrhPasswordDefault),
				OSSRHUsername:     formatWorkflowSecret(osshrUsernameDefault),
				UseSonatypeLegacy: sonatypeLegacy,
			},
		}
	case "terraform":
		target.Publishing = &workflow.Publishing{
			Terraform: &workflow.Terraform{
				GPGPrivateKey: formatWorkflowSecret(terraformGPGPrivateKeyDefault),
				GPGPassPhrase: formatWorkflowSecret(terraformGPGPassPhraseDefault),
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
			input = charm.NewInlineInput(value).
				Title(fmt.Sprintf("Provide a name for your %s secret:", prompt.key))
		} else {
			input = charm.NewInlineInput(value).
				Title(fmt.Sprintf("Provide the value of your %s:", prompt.key))
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
		charm.WithTitle(fmt.Sprintf("Setup publishing variables for your %s target %s.", target.Target, name)),
		charm.WithDescription("These environment variables will be used to publish to package managers from your speakeasy workflow.")).
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
		secrets = append(secrets, publishing.Packagist.Username)
	}

	if publishing.Java != nil {
		secrets = append(secrets, publishing.Java.GPGSecretKey)
		secrets = append(secrets, publishing.Java.GPGPassPhrase)
		secrets = append(secrets, publishing.Java.OSSHRPassword)
		secrets = append(secrets, publishing.Java.OSSRHUsername)
	}

	if publishing.Terraform != nil {
		secrets = append(secrets, publishing.Terraform.GPGPrivateKey)
		secrets = append(secrets, publishing.Terraform.GPGPassPhrase)
	}

	return secrets
}

func WriteTestingFiles(ctx context.Context, wf *workflow.Workflow, currentWorkingDir, workflowFileDir string, selectedTargets []string, isPatBased bool) ([]string, error) {
	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
	secrets[config.SpeakeasyApiKey] = formatGithubSecretName(defaultSpeakeasyAPIKeySecretName)
	var filePaths []string
	// Write the appropriate testing files
	for _, name := range selectedTargets {
		testingFile := defaultTestingFile(name, wf.Targets[name].Output, workflowFileDir, secrets)
		filePath := filepath.Join(currentWorkingDir, ".github/workflows/sdk_test.yaml")
		if len(wf.Targets) > 1 {
			testingFile.Name = fmt.Sprintf("Test %s", strings.ToUpper(name))
			sanitizedName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
			filePath = filepath.Join(currentWorkingDir, fmt.Sprintf(".github/workflows/sdk_test_%s.yaml", sanitizedName))
		}

		if err := writeTestingFile(testingFile, filePath); err != nil {
			return nil, err
		}

		filePaths = append(filePaths, filePath)
	}

	// Attempt to update the appropriate generation workflow if they choose a PAT based approach
	if isPatBased {
		for name := range wf.Targets {
			generationWorkflow := &config.GenerateWorkflow{}
			generationWorkflowFilePath := filepath.Join(currentWorkingDir, ".github/workflows/sdk_generation.yaml")
			if len(wf.Targets) > 1 {
				sanitizedName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
				generationWorkflowFilePath = filepath.Join(currentWorkingDir, fmt.Sprintf(".github/workflows/sdk_generation_%s.yaml", sanitizedName))
			}
			if err := ReadGenerationFile(generationWorkflow, generationWorkflowFilePath); err == nil {
				generationWorkflow.Jobs.Generate.Secrets["pr_creation_pat"] = formatGithubSecretName("pr_creation_pat")
			}

			if err := WriteGenerationFile(generationWorkflow, generationWorkflowFilePath); err != nil {
				log.From(ctx).Warnf("failed to to update %s with pr_creation_pat", generationWorkflowFilePath)
			}
		}
	}

	return filePaths, nil
}

// WritePublishing writes a github action file for a given target for publishing to a package manager.
// If filenameAddendum is provided, it will be appended to the filename (i.e. sdk_publish_lending.yaml).
// Returns the paths to the files written.
func WritePublishing(wf *workflow.Workflow, genWorkflow *config.GenerateWorkflow, targetName, currentWorkingDir, workflowFileDir string, target workflow.Target) ([]string, error) {
	secrets := make(map[string]string)
	secrets[config.GithubAccessToken] = formatGithubSecretName(defaultGithubTokenSecretName)
	secrets[config.SpeakeasyApiKey] = formatGithubSecretName(defaultSpeakeasyAPIKeySecretName)

	var terraformOutDir *string

	if target.Publishing != nil {
		for _, secret := range getSecretsValuesFromPublishing(*target.Publishing) {
			secrets[formatGithubSecret(secret)] = formatGithubSecretName(secret)
		}

		if target.Target == "terraform" {
			terraformOutDir = target.Output
		}
	}

	currentSecrets := genWorkflow.Jobs.Generate.Secrets
	for secret, value := range secrets {
		currentSecrets[secret] = value
	}
	genWorkflow.Jobs.Generate.Secrets = currentSecrets

	mode := genWorkflow.Jobs.Generate.With[config.Mode].(string)
	if target.Target == "terraform" {
		releaseActionPath := filepath.Join(currentWorkingDir, ".github/workflows/tf_provider_release.yaml")
		goReleaserPath := currentWorkingDir
		if terraformOutDir != nil {
			goReleaserPath = filepath.Join(goReleaserPath, filepath.Join(workflowFileDir, *terraformOutDir))
		}
		goReleaserPath = filepath.Join(goReleaserPath, ".goreleaser.yml")
		releasePaths := []string{releaseActionPath, goReleaserPath}
		if err := os.WriteFile(releaseActionPath, []byte(terraformReleaseAction), 0o644); err != nil {
			return releasePaths, errors.Wrapf(err, "failed to write terraform release github action release file %s", terraformReleaseAction)
		}

		if err := os.WriteFile(goReleaserPath, []byte(goReleaser), 0o644); err != nil {
			return releasePaths, errors.Wrapf(err, "failed to write terraform goreleaser file %s", goReleaserPath)
		}

		return releasePaths, nil
	} else if mode == "pr" {
		filePath := filepath.Join(currentWorkingDir, ".github/workflows/sdk_publish.yaml")
		if len(wf.Targets) > 1 {
			sanitizedName := strings.ReplaceAll(strings.ToLower(targetName), "-", "_")
			filePath = filepath.Join(currentWorkingDir, fmt.Sprintf(".github/workflows/sdk_publish_%s.yaml", sanitizedName))
		}

		publishingFile := &config.PublishWorkflow{}
		if err := readPublishingFile(publishingFile, filePath); err != nil {
			publishingFile = defaultPublishingFile()
		}

		// backfill id-token write permissions
		if publishingFile.Permissions.IDToken != config.GithubWritePermission {
			publishingFile.Permissions.IDToken = config.GithubWritePermission
		}

		if len(wf.Targets) > 1 {
			publishingFile.Name = fmt.Sprintf("Publish %s", strings.ToUpper(targetName))
		}

		configDirectory := workflowFileDir
		if outputPath := target.Output; outputPath != nil {
			trimmedPath := strings.TrimPrefix(*outputPath, "./")
			configDirectory = filepath.Join(configDirectory, trimmedPath)
		}

		publishingFile.On.Push.Paths = []string{filepath.Join(configDirectory, ".speakeasy/gen.lock")}
		if publishingFile.Jobs.Publish.With == nil {
			publishingFile.Jobs.Publish.With = make(map[string]interface{})
		}

		publishingFile.Jobs.Publish.With["target"] = targetName

		if workflowFileDir != "" {
			publishingFile.Jobs.Publish.With["working_directory"] = workflowFileDir
		}

		for name, value := range secrets {
			publishingFile.Jobs.Publish.Secrets[name] = value
		}

		// Write a github publishing file.
		var publishingWorkflowBuf bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&publishingWorkflowBuf)
		yamlEncoder.SetIndent(2)
		if err := yamlEncoder.Encode(publishingFile); err != nil {
			return nil, errors.Wrapf(err, "failed to encode workflow file")
		}

		if err := os.WriteFile(filePath, publishingWorkflowBuf.Bytes(), 0o644); err != nil {
			return []string{filePath}, errors.Wrapf(err, "failed to write github publishing file")
		}

		return []string{filePath}, nil
	}

	return nil, nil
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

func writeTestingFile(testingWorkflow *config.TestingWorkflow, testingWorkflowFilePath string) error {
	var genWorkflowBuf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&genWorkflowBuf)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(testingWorkflow); err != nil {
		return errors.Wrapf(err, "failed to encode workflow file")
	}

	if err := os.WriteFile(testingWorkflowFilePath, genWorkflowBuf.Bytes(), 0o644); err != nil {
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
			PullRequest: config.PullRequestOn{
				Types: []string{"labeled", "unlabeled"},
			},
		},
		Jobs: config.Jobs{
			Generate: config.Job{
				Uses: "speakeasy-api/sdk-generation-action/.github/workflows/workflow-executor.yaml@v15",
				With: map[string]any{
					"force":     "${{ github.event.inputs.force }}",
					config.Mode: "pr",
				},
				Secrets: secrets,
			},
		},
		Permissions: config.Permissions{
			Checks:       config.GithubWritePermission,
			Statuses:     config.GithubWritePermission,
			Contents:     config.GithubWritePermission,
			PullRequests: config.GithubWritePermission,
			IDToken:      config.GithubWritePermission,
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
			IDToken:      config.GithubWritePermission,
		},
		On: config.PublishOn{
			Push: config.Push{
				Paths: []string{},
				Branches: []string{
					"main",
				},
			},
			WorkflowDispatch: &config.WorkflowDispatchEmpty{},
		},
		Jobs: config.Jobs{
			Publish: config.Job{
				Uses:    "speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15",
				With:    make(map[string]any),
				Secrets: make(map[string]string),
			},
		},
	}
}

func defaultTestingFile(sdkName string, sdkOutputDir *string, workflowFileDir string, secrets map[string]string) *config.TestingWorkflow {
	sdkPath := "**"
	if sdkOutputDir != nil && *sdkOutputDir != "." && *sdkOutputDir != "./" {
		sdkPath = fmt.Sprintf("%s/**", filepath.Join(workflowFileDir, strings.TrimPrefix(*sdkOutputDir, "/")))
	}
	testingAction := &config.TestingWorkflow{
		Name: "Test",
		Permissions: config.Permissions{
			Checks:       config.GithubWritePermission,
			Statuses:     config.GithubWritePermission,
			Contents:     config.GithubWritePermission,
			PullRequests: config.GithubWritePermission,
			IDToken:      config.GithubWritePermission,
		},
		On: config.TestingOn{
			PullRequest: config.Push{
				Paths: []string{
					sdkPath,
				},
				Branches: []string{
					"main",
				},
			},
			WorkflowDispatch: config.WorkflowDispatchTesting{
				Inputs: config.InputsTesting{
					Target: config.Target{
						Description: "Provided SDK target to run tests for, (all) is valid",
						Type:        "string",
					},
				},
			},
		},
		Jobs: config.Jobs{
			Test: config.Job{
				Uses: "speakeasy-api/sdk-generation-action/.github/workflows/sdk-test.yaml@v15",
				With: map[string]any{
					"target": fmt.Sprintf("${{ github.event.inputs.target || '%s' }}", sdkName),
				},
				Secrets: secrets,
			},
		},
	}

	if workflowFileDir != "" {
		testingAction.Jobs.Test.With["working_directory"] = workflowFileDir
	}

	return testingAction
}

func SelectPublishingTargets(publishingOptions []huh.Option[string], autoSelect bool) ([]string, error) {
	chosenTargets := make([]string, 0)
	if autoSelect {
		for _, option := range publishingOptions {
			chosenTargets = append(chosenTargets, option.Value)
		}
	}

	form := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select targets to configure publishing configs for.").
			Description("Setup variables to configure publishing directly from Speakeasy.\n").
			Options(publishingOptions...).
			Value(&chosenTargets),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := form.ExecuteForm(); err != nil {
		return nil, err
	}

	return chosenTargets, nil
}

func SelectTestingTargets(testingOptions []huh.Option[string], autoSelect bool) ([]string, error) {
	chosenTargets := make([]string, 0)
	if autoSelect {
		for _, option := range testingOptions {
			chosenTargets = append(chosenTargets, option.Value)
		}
	}

	form := charm.NewForm(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select targets to configure sdk tests for.").
			Description("Bootstrap tests for Speakeasy SDKs.\n").
			Options(testingOptions...).
			Value(&chosenTargets),
	)), charm.WithKey("x/space", "toggle"))

	if _, err := form.ExecuteForm(); err != nil {
		return nil, err
	}

	return chosenTargets, nil
}
