package cmd

import (
	"context"
	"fmt"
	config "github.com/speakeasy-api/sdk-gen-config"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/internal/model"
	"github.com/speakeasy-api/speakeasy/internal/model/flag"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
	"strings"
)

const defaultSourceName = "my-source"

type MigrateFlags struct {
	Directory   string `json:"directory"`
	GenFilename string `json:"gen-filename"`
}

var migrateCmd = &model.ExecutableCommand[MigrateFlags]{
	Usage:  "migrate",
	Short:  "migrate to v15 of the speakeasy workflow + action",
	Long:   "migrate to v15 of the speakeasy workflow + action",
	Hidden: true,
	Run:    migrateFunc,
	Flags: []flag.Flag{
		flag.StringFlag{
			Name:         "directory",
			Shorthand:    "d",
			Description:  "directory to migrate. Expected to contain a `.github/workflows` directory. Defaults to `.`",
			DefaultValue: ".",
		},
		flag.StringFlag{
			Name:         "gen-filename",
			Shorthand:    "g",
			Description:  "name of the file containing the generation workflow. Defaults to `speakeasy_sdk_generation.yml`",
			DefaultValue: "speakeasy_sdk_generation.yml",
		},
	},
}

func migrateFunc(ctx context.Context, flags MigrateFlags) error {
	// We intentionally don't do anything with the publish workflow. It can stay the same

	if b, _ := os.ReadFile(fmt.Sprintf("%s/.speakeasy/workflow.yaml", flags.Directory)); b != nil {
		return fmt.Errorf("a workflow.yaml file already exists in the .speakeasy directory. Aborting migration")
	}

	currentGhWorkflow, err := os.ReadFile(fmt.Sprintf("%s/.github/workflows/%s", flags.Directory, flags.GenFilename))
	if err != nil {
		return err
	}
	workflow, ghWorkflow, err := parseWorkflowFiles(string(currentGhWorkflow))
	if err != nil {
		return err
	}

	ghWorkflowYaml, err := yaml.Marshal(ghWorkflow)
	if err != nil {
		return err
	}

	workflowYaml, err := yaml.Marshal(workflow)
	if err != nil {
		return err
	}

	if err := os.Mkdir(fmt.Sprintf("%s/.speakeasy", flags.Directory), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf("%s/.speakeasy/workflow.yaml", flags.Directory), workflowYaml, 0644); err != nil {
		return err
	}

	// Write this one last since it overwrites the existing file
	if err := os.WriteFile(fmt.Sprintf("%s/.github/workflows/%s", flags.Directory, flags.GenFilename), ghWorkflowYaml, 0644); err != nil {
		return err
	}

	return nil
}

func parseWorkflowFiles(genWorkflow string) (*workflow.Workflow, *config.GenerateWorkflow, error) {
	generationWorkflow := config.GenerateWorkflow{}
	if err := yaml.Unmarshal([]byte(genWorkflow), &generationWorkflow); err != nil {
		return nil, nil, err
	}

	/*
	 * CREATE THE NEW GITHUB WORKFLOW FILE
	 */

	mode := "direct"
	if m, ok := generationWorkflow.Jobs.Generate.With["mode"]; ok {
		mode = m.(string)
	}

	newGenWorkflow := &config.GenerateWorkflow{
		Name:        generationWorkflow.Name,
		On:          generationWorkflow.On,
		Permissions: generationWorkflow.Permissions,
		Jobs: config.Jobs{
			Generate: config.Job{
				Uses: "speakeasy-api/sdk-generation-action-v15/.github/workflows/workflow-executor.yaml@v15",
				With: map[string]any{
					"force":             generationWorkflow.Jobs.Generate.With["force"],
					"speakeasy_version": generationWorkflow.Jobs.Generate.With["speakeasy_version"],
					"mode":              mode,
				},
				Secrets: generationWorkflow.Jobs.Generate.Secrets,
			},
		},
	}

	/*
	 * CREATE SOURCES
	 */

	docLocations := []string{}
	if docs, ok := generationWorkflow.Jobs.Generate.With["openapi_docs"]; ok {
		docLocations = append(docLocations, docs.([]string)...)
	} else if docLocation, ok := generationWorkflow.Jobs.Generate.With["openapi_doc_location"]; ok {
		docLocations = append(docLocations, docLocation.(string))
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
		targets[fmt.Sprintf("%s-target", lang)] = workflow.Target{
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
			Secret: "$OPENAPI_DOC_AUTH_TOKEN", // This is the name of the secret in the action, which is fixed
		}
	}

	var documents []workflow.Document
	for _, docLocation := range docLocations {
		documents = append(documents, workflow.Document{
			Location: docLocation,
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
					OSSRHUsername: "$OSSRH_USERNAME",
					OSSHRPassword: "$OSSRH_PASSWORD",
					GPGSecretKey:  "$JAVA_GPG_SECRET_KEY",
					GPGPassPhrase: "$JAVA_GPG_PASSPHRASE",
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
