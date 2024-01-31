package prompts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
)

func getBaseSourcePrompts(currentWorkflow *workflow.Workflow, sourceName, fileLocation, authHeader, authSecret *string) []*huh.Group {
	var initialGroup []huh.Field

	if sourceName == nil || *sourceName == "" {
		initialGroup = append(initialGroup,
			charm_internal.NewInput().
				Title("What is a good name for this source?").
				Validate(func(s string) error {
					if _, ok := currentWorkflow.Sources[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(sourceName),
		)
	}

	if fileLocation == nil || *fileLocation == "" {
		initialGroup = append(initialGroup,
			charm_internal.NewInput().
				Title("What is the location of your OpenAPI document?").
				Placeholder("local file path or remote file reference.").
				Value(fileLocation).
				Suggestions(schemaFilesInCurrentDir()),
		)
	}

	var groups []*huh.Group

	if len(initialGroup) > 0 {
		groups = append(groups, huh.NewGroup(initialGroup...))
	}

	groups = append(groups, getRemoteAuthenticationPrompts(fileLocation, authHeader, authSecret)...)
	return groups
}

func getRemoteAuthenticationPrompts(fileLocation, authHeader, authSecret *string) []*huh.Group {
	requiresAuthentication := false
	return []*huh.Group{
		huh.NewGroup(
			huh.NewConfirm().
				Title("Does this remote file require authentication?").
				Affirmative("Yes.").
				Negative("No.").
				Value(&requiresAuthentication),
		).WithHideFunc(func() bool {
			if fileLocation != nil && *fileLocation != "" {
				if parsedUrl, err := url.ParseRequestURI(*fileLocation); err == nil {
					resp, err := http.Get(parsedUrl.String())
					if err != nil {
						return false
					} else {
						defer resp.Body.Close()

						if resp.StatusCode < 200 || resp.StatusCode > 299 {
							return false
						}
					}
				}
			}
			return true
		}),
		huh.NewGroup(
			charm_internal.NewInput().
				Title("What is the name of your authentication Header?").
				Placeholder("x-auth-token").
				Value(authHeader),
			charm_internal.NewInput().
				Title("What is the reference to your auth secret?").
				Placeholder("$AUTH_TOKEN").
				Value(authSecret),
		).WithHideFunc(func() bool {
			return !requiresAuthentication
		}),
	}
}

func getOverlayPrompts(promptForOverlay *bool, overlayLocation, authHeader, authSecret *string) []*huh.Group {
	groups := []*huh.Group{
		huh.NewGroup(
			charm_internal.NewInput().
				Title("What is the location of your Overlay file?").
				Placeholder("local file path or remote file reference.").
				Value(overlayLocation).
				Suggestions(schemaFilesInCurrentDir()),
		).WithHideFunc(func() bool {
			return !*promptForOverlay
		}),
	}

	groups = append(groups, getRemoteAuthenticationPrompts(overlayLocation, authHeader, authSecret)...)
	return groups
}

func sourceBaseForm(quickstart *Quickstart) (*QuickstartState, error) {
	source := &workflow.Source{}
	var sourceName, fileLocation, authHeader, authSecret string
	if len(quickstart.WorkflowFile.Sources) == 0 {
		sourceName = "openapi"
	}

	if quickstart.Defaults.SchemaPath != nil {
		fileLocation = *quickstart.Defaults.SchemaPath
	}

	if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
		getBaseSourcePrompts(quickstart.WorkflowFile, &sourceName, &fileLocation, &authHeader, &authSecret)...),
		"Let's setup a new source for your workflow.",
		"A source is a compiled set of OpenAPI specs and overlays that are used as the input for a SDK generation.")).
		Run(); err != nil {
		return nil, err
	}

	document, err := formatDocument(fileLocation, authHeader, authSecret, false)
	if err != nil {
		return nil, err
	}

	source.Inputs = append(source.Inputs, *document)

	if err := source.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate source")
	}

	quickstart.WorkflowFile.Sources[sourceName] = *source

	nextState := TargetBase

	return &nextState, nil
}

func AddToSource(name string, currentSource *workflow.Source) (*workflow.Source, error) {
	addOpenAPIFile := false
	if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
		charm_internal.NewBranchPrompt("Would you like to add an openapi file to this source?", &addOpenAPIFile)),
		fmt.Sprintf("Let's add to the source %s", name))).
		Run(); err != nil {
		return nil, err
	}

	for addOpenAPIFile {
		addOpenAPIFile = false
		var fileLocation, authHeader, authSecret string
		groups := []*huh.Group{
			huh.NewGroup(
				charm_internal.NewInput().
					Title("What is the location of your OpenAPI document?").
					Placeholder("local file path or remote file reference.").
					Value(&fileLocation),
			),
		}
		groups = append(groups, getRemoteAuthenticationPrompts(&fileLocation, &authHeader, &authSecret)...)
		groups = append(groups, charm_internal.NewBranchPrompt("Would you like to add another openapi file to this source?", &addOpenAPIFile))
		if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
			groups...),
			fmt.Sprintf("Let's add to the source %s", name))).
			Run(); err != nil {
			return nil, err
		}
		document, err := formatDocument(fileLocation, authHeader, authSecret, true)
		if err != nil {
			return nil, err
		}

		currentSource.Inputs = append(currentSource.Inputs, *document)
	}

	addOverlayFile := false
	if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
		charm_internal.NewBranchPrompt("Would you like to add an overlay file to this source?", &addOverlayFile)),
		fmt.Sprintf("Let's add to the source %s", name))).
		Run(); err != nil {
		return nil, err
	}

	for addOverlayFile {
		addOverlayFile = false
		var fileLocation, authHeader, authSecret string
		trueVal := true
		groups := getOverlayPrompts(&trueVal, &fileLocation, &authHeader, &authSecret)
		groups = append(groups, charm_internal.NewBranchPrompt("Would you like to add another overlay file to this source?", &addOverlayFile))
		if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
			groups...),
			fmt.Sprintf("Let's add to the source %s", name))).
			Run(); err != nil {
			return nil, err
		}
		document, err := formatDocument(fileLocation, authHeader, authSecret, true)
		if err != nil {
			return nil, err
		}

		currentSource.Overlays = append(currentSource.Overlays, *document)
	}

	return currentSource, nil
}

func PromptForNewSource(currentWorkflow *workflow.Workflow) (string, *workflow.Source, error) {
	source := &workflow.Source{}
	var sourceName, fileLocation, authHeader, authSecret string
	var overlayFileLocation, overlayAuthHeader, overlayAuthSecret, outputLocation string

	groups := getBaseSourcePrompts(currentWorkflow, &sourceName, &fileLocation, &authHeader, &authSecret)
	var promptForOverlay bool
	groups = append(groups, charm_internal.NewBranchPrompt("Would you like to add an overlay file to this source?", &promptForOverlay))
	groups = append(groups, getOverlayPrompts(&promptForOverlay, &overlayFileLocation, &overlayAuthHeader, &overlayAuthSecret)...)
	groups = append(groups, huh.NewGroup(
		charm_internal.NewInput().
			Title("Optionally provide an output location for your build source file:").
			Placeholder("output.yaml").
			Value(&outputLocation).
			Suggestions(schemaFilesInCurrentDir()),
	).WithHideFunc(
		func() bool {
			return len(currentWorkflow.Sources) == 0
		}))

	if _, err := tea.NewProgram(charm_internal.NewForm(huh.NewForm(
		groups...),
		"Let's setup a new source for your workflow.",
		"A source is a compiled set of OpenAPI specs and overlays that are used as the input for a SDK generation.")).
		Run(); err != nil {
		return "", nil, err
	}

	document, err := formatDocument(fileLocation, authHeader, authSecret, false)
	if err != nil {
		return "", nil, err
	}

	source.Inputs = append(source.Inputs, *document)

	if overlayFileLocation != "" {
		document, err := formatDocument(overlayFileLocation, overlayAuthHeader, overlayAuthSecret, false)
		if err != nil {
			return "", nil, err
		}

		source.Overlays = append(source.Overlays, *document)
	}

	if outputLocation != "" {
		source.Output = &outputLocation
	}

	if err := source.Validate(); err != nil {
		return "", nil, errors.Wrap(err, "failed to validate source")
	}

	return sourceName, source, nil
}

func formatDocument(fileLocation, authHeader, authSecret string, validate bool) (*workflow.Document, error) {
	document := &workflow.Document{
		Location: fileLocation,
	}

	if authHeader != "" && authSecret != "" {
		document.Auth = &workflow.Auth{
			Header: authHeader,
			Secret: authSecret,
		}
	}

	if validate {
		if err := document.Validate(); err != nil {
			return nil, errors.Wrap(err, "failed to validate new document")
		}
	}

	return document, nil
}

// Populates tab complete for schema files in the current directory
func schemaFilesInCurrentDir() []string {
	var validFiles []string
	dir, err := os.Getwd()
	if err != nil {
		return validFiles
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return validFiles
	}

	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".json")) {
			validFiles = append(validFiles, file.Name())
		}
	}

	return validFiles
}
