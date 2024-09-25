package migrate

import (
	"context"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"gopkg.in/yaml.v3"
)

const defaultSourceName = "my-source"

func Migrate(ctx context.Context, directory string) error {
	logger := log.From(ctx)

	if b, _ := os.ReadFile(fmt.Sprintf("%s/.speakeasy/workflow.yaml", directory)); b != nil {
		return fmt.Errorf("a workflow.yaml file already exists in the .speakeasy directory. Aborting migration")
	}

	/*
	 * Try to find the current workflow files
	 */

	dir, err := os.ReadDir(fmt.Sprintf("%s/.github/workflows", directory))
	if err != nil {
		return err
	}

	var currentGenWorkflow, genWorkflowFilename, currentPubWorkflow, pubWorkflowFilename string
	for _, file := range dir {
		if !file.IsDir() {
			fileBytes, err := os.ReadFile(fmt.Sprintf("%s/.github/workflows/%s", directory, file.Name()))
			if err != nil {
				continue
			}

			fileContents := string(fileBytes)
			if strings.Contains(fileContents, "speakeasy-api/sdk-generation-action/.github/workflows/sdk-generation.yaml") {
				currentGenWorkflow = fileContents
				genWorkflowFilename = file.Name()

				logger.Infof("Found the current generation workflow file: %s\n", file.Name())
			} else if strings.Contains(fileContents, "speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml") {
				currentPubWorkflow = fileContents
				pubWorkflowFilename = file.Name()

				logger.Infof("Found the current publishing workflow file: %s\n", file.Name())
			}

			if currentGenWorkflow != "" && currentPubWorkflow != "" {
				break
			}
		}
	}

	if currentGenWorkflow == "" {
		return fmt.Errorf("could not find the existing generation workflow file")
	} else if currentPubWorkflow == "" {
		logger.Infof("Could not find the existing publishing workflow file. Assuming generation workflow is in direct mode.")
	}

	/*
	 * Convert the current workflow files to the new format
	 */

	workflow, genActionWorkflow, err := buildGenerationWorkflowFiles(currentGenWorkflow)
	if err != nil {
		return err
	}

	pubActionWorkflow, err := buildNewPublishingWorkflow(currentPubWorkflow)
	if err != nil {
		return err
	}

	// Special case: pull use_sonatype_central from the publishing workflow
	if strings.Contains(currentPubWorkflow, "use_sonatype_central: true") {
		for _, target := range workflow.Targets {
			if target.Target == "java" {
				target.Publishing.Java.UseSonatypeLegacy = true
			}
		}
	}

	workflowYaml, err := yaml.Marshal(workflow)
	if err != nil {
		return err
	}

	genActionWorkflowYaml, err := yaml.Marshal(genActionWorkflow)
	if err != nil {
		return err
	}

	pubActionWorkflowYaml, err := yaml.Marshal(pubActionWorkflow)
	if err != nil {
		return err
	}

	if err := os.Mkdir(fmt.Sprintf("%s/.speakeasy", directory), 0o755); err != nil && !os.IsExist(err) {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf("%s/.speakeasy/workflow.yaml", directory), workflowYaml, 0o644); err != nil {
		return err
	}

	// Write these last since they overwrite the existing files
	if err := os.WriteFile(fmt.Sprintf("%s/.github/workflows/%s", directory, genWorkflowFilename), genActionWorkflowYaml, 0o644); err != nil {
		return err
	}

	if pubActionWorkflow != nil && pubWorkflowFilename != "" {
		if err := os.WriteFile(fmt.Sprintf("%s/.github/workflows/%s", directory, pubWorkflowFilename), pubActionWorkflowYaml, 0o644); err != nil {
			return err
		}
	}

	status := []string{
		fmt.Sprintf("Speakeasy workflow written to - %s/.speakeasy/workflow.yaml", directory),
		fmt.Sprintf("GitHub action (generate) written to - %s/.github/workflows/%s", directory, genWorkflowFilename),
	}
	if pubActionWorkflow != nil && pubWorkflowFilename != "" {
		status = append(status, fmt.Sprintf("GitHub action (publish) written to - %s/.github/workflows/%s", directory, pubWorkflowFilename))
	}

	status = append(status, "The following openapi specs are currently included:")
	for _, source := range workflow.Sources {
		for _, spec := range source.Inputs {
			status = append(status, fmt.Sprintf("\t• %s", spec.Location))
		}
	}

	status = append(status, "The following languages are configured:")
	for _, target := range workflow.Targets {
		status = append(status, fmt.Sprintf("\t• %s", target.Target))
	}
	status = append(status, "Try out speakeasy run to regenerate your SDK locally!")

	logger.Println(styles.RenderInstructionalMessage("Successfully migrated to the new workflow format!", status...))

	return nil
}

func buildGenerationWorkflowFiles(genWorkflow string) (*workflow.Workflow, *config.GenerateWorkflow, error) {
	generationWorkflow := config.GenerateWorkflow{}
	if err := yaml.Unmarshal([]byte(genWorkflow), &generationWorkflow); err != nil {
		return nil, nil, err
	}

	/*
	 * CREATE THE NEW GITHUB WORKFLOW FILE
	 */

	with := copyNonemptyMapValues(generationWorkflow.Jobs.Generate.With, "force", "speakeasy_version", "mode")
	if _, ok := with["mode"]; !ok {
		with["mode"] = "pr"
	}

	newGenWorkflow := &config.GenerateWorkflow{
		Name:        generationWorkflow.Name,
		On:          generationWorkflow.On,
		Permissions: generationWorkflow.Permissions,
		Jobs: config.Jobs{
			Generate: config.Job{
				Uses:    "speakeasy-api/sdk-generation-action/.github/workflows/workflow-executor.yaml@v15",
				With:    with,
				Secrets: generationWorkflow.Jobs.Generate.Secrets,
			},
		},
	}

	/*
	 * CREATE SOURCES
	 */

	docLocations := []string{}
	if docs, ok := generationWorkflow.Jobs.Generate.With["openapi_docs"]; ok {
		var items []string
		err := yaml.Unmarshal([]byte(docs.(string)), &items)
		if err != nil {
			return nil, nil, fmt.Errorf("openapi_docs must be an array: %d", err)
		}

		docLocations = append(docLocations, items...)
	} else if docLocation, ok := generationWorkflow.Jobs.Generate.With["openapi_doc_location"]; ok {
		if docLocationString, ok := docLocation.(string); ok {
			docLocations = append(docLocations, docLocationString)
		} else {
			return nil, nil, fmt.Errorf("openapi_doc_location must be a string")
		}
	}

	header := ""
	if auth, ok := generationWorkflow.Jobs.Generate.With["openapi_doc_auth_header"]; ok {
		header = auth.(string)
	}
	_, hasToken := generationWorkflow.Jobs.Generate.Secrets["openapi_doc_auth_token"]

	sources := map[string]workflow.Source{
		defaultSourceName: {
			Inputs: docLocationsToDocuments(docLocations, header, hasToken),
		},
	}

	/*
	 * CREATE TARGETS
	 */

	languages := generationWorkflow.Jobs.Generate.With["languages"].(string)
	langToOutput, err := parseLanguages(languages)
	if err != nil {
		return nil, nil, err
	}

	targets := map[string]workflow.Target{}
	for lang, output := range langToOutput {
		name := promptForTargetName(lang, slices.Collect(maps.Keys(targets)))

		targets[name] = workflow.Target{
			Source:     defaultSourceName,
			Target:     lang,
			Output:     output,
			Publishing: getPublishing(generationWorkflow, lang),
		}
	}

	workflowFile := &workflow.Workflow{
		Version: "1.0.0",
		Sources: sources,
		Targets: targets,
	}

	return workflowFile, newGenWorkflow, nil
}

func promptForTargetName(lang string, existingNames []string) string {
	validateUnique := func(input string) error {
		if slices.Contains(existingNames, input) {
			return fmt.Errorf("target name must be unique within this workflow")
		}
		return nil
	}

	form := interactivity.NewSimpleInput(interactivity.InputField{
		Name:        fmt.Sprintf("Choose a name to identify your %s generation target", lang),
		Placeholder: fmt.Sprintf("My %s Target", utils.CapitalizeFirst(lang)),
	}, validateUnique)

	return form.Run()
}

func buildNewPublishingWorkflow(pubWorkflow string) (*config.PublishWorkflow, error) {
	publishingWorkflow := config.PublishWorkflow{}
	if err := yaml.Unmarshal([]byte(pubWorkflow), &publishingWorkflow); err != nil {
		return nil, err
	}

	newPublishingWorkflow := &config.PublishWorkflow{
		Name: publishingWorkflow.Name,
		Permissions: config.Permissions{
			Checks:       config.GithubWritePermission,
			Statuses:     config.GithubWritePermission,
			Contents:     config.GithubWritePermission,
			PullRequests: config.GithubWritePermission,
			IDToken:      config.GithubWritePermission,
		},
		On: publishingWorkflow.On,
		Jobs: config.Jobs{
			Publish: config.Job{
				Uses:    "speakeasy-api/sdk-generation-action/.github/workflows/sdk-publish.yaml@v15",
				With:    copyNonemptyMapValues(publishingWorkflow.Jobs.Publish.With, "dotnet_version"),
				Secrets: publishingWorkflow.Jobs.Publish.Secrets,
			},
		},
	}

	return newPublishingWorkflow, nil
}

func copyNonemptyMapValues(src map[string]any, keysToCopy ...string) map[string]any {
	dst := map[string]any{}
	for k, v := range src {
		if slices.Contains(keysToCopy, k) && v != nil && v != "" {
			dst[k] = v
		}
	}
	return dst
}

func parseLanguages(langString string) (map[string]*string, error) {
	languageElems, err := getAllSubmatches(regexp.MustCompile(`\s*?- (.*)`), langString)
	if err != nil {
		return nil, err
	}

	languageToSubdirectory := map[string]*string{}
	for _, languageElem := range languageElems {
		// Languages are in one of the following formats:
		// - go: ./path-to-go
		// - go
		if strings.Contains(languageElem, ":") {
			languageAndSubdirectory := strings.Split(languageElem, ": ")
			if len(languageAndSubdirectory) != 2 { //nolint:gomnd
				return nil, fmt.Errorf("incorrectly formatted language in workflow file. Culprit: %s", languageElem)
			}
			languageToSubdirectory[languageAndSubdirectory[0]] = &languageAndSubdirectory[1]
		} else {
			languageToSubdirectory[languageElem] = nil
		}
	}

	return languageToSubdirectory, nil
}

func docLocationsToDocuments(docLocations []string, authHeader string, hasToken bool) []workflow.Document {
	var auth *workflow.Auth
	if hasToken {
		auth = &workflow.Auth{
			Header: authHeader,
			Secret: "$openapi_doc_auth_token", // This is the name of the secret in the action, which is fixed
		}
	}

	var documents []workflow.Document
	for _, docLocation := range docLocations {
		documents = append(documents, workflow.Document{
			Location: workflow.LocationString(docLocation),
			Auth:     auth,
		})
	}
	return documents
}

func getPublishing(genWorkflow config.GenerateWorkflow, lang string) *workflow.Publishing {
	pubVal, ok := genWorkflow.Jobs.Generate.With[fmt.Sprintf("publish_%s", lang)]
	shouldPublish := ok && pubVal.(bool)

	if shouldPublish {
		// These secret values are hardcoded because they are the names of the secrets in the action
		switch lang {
		case "typescript":
			return &workflow.Publishing{
				NPM: &workflow.NPM{
					Token: "$NPM_TOKEN",
				},
			}
		case "python":
			return &workflow.Publishing{
				PyPi: &workflow.PyPi{
					Token: "$PYPI_TOKEN",
				},
			}
		case "php":
			return &workflow.Publishing{
				Packagist: &workflow.Packagist{
					Username: "$PACKAGIST_USERNAME",
					Token:    "$PACKAGIST_TOKEN",
				},
			}
		case "java":
			return &workflow.Publishing{
				Java: &workflow.Java{
					OSSRHUsername:     "$OSSRH_USERNAME",
					OSSHRPassword:     "$OSSRH_PASSWORD",
					GPGSecretKey:      "$JAVA_GPG_SECRET_KEY",
					GPGPassPhrase:     "$JAVA_GPG_PASSPHRASE",
					UseSonatypeLegacy: true, // Default to true for backwards compatibility
				},
			}
		case "ruby":
			return &workflow.Publishing{
				RubyGems: &workflow.RubyGems{
					Token: "$RUBYGEMS_AUTH_TOKEN",
				},
			}
		case "csharp":
			return &workflow.Publishing{
				Nuget: &workflow.Nuget{
					APIKey: "$NUGET_API_KEY",
				},
			}
		}
	}

	return nil
}

func getAllSubmatches(re *regexp.Regexp, s string) ([]string, error) {
	// Finds all matches. Returns an array of arrays. In each subarray:
	// - match[0] will contain the full regex match
	// - match[1] will contain the captured group (submatch)
	matches := re.FindAllStringSubmatch(s, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf(
			"could not find match. Regex: %s, matching against: %s",
			re.String(),
			s,
		)
	}

	submatches := make([]string, len(matches))
	for i, match := range matches {
		if len(match) <= 1 {
			return nil, fmt.Errorf(
				"missing submatch. Regex %s, matching against: %s",
				re.String(),
				s,
			)
		}

		submatches[i] = match[1]
	}

	return submatches, nil
}
