package quickstart

import (
	"fmt"
	"net/url"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
	"github.com/speakeasy-api/speakeasy/charm"
)

func sourceBaseForm(inputWorkflow *workflow.Workflow) (*State, error) {
	source := &workflow.Source{}
	var sourceName string
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("A name for this source:").
				Placeholder("unique name across this workflow").
				Prompt(" ").
				Inline(true).
				Validate(func(s string) error {
					if _, ok := inputWorkflow.Sources[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(&sourceName),
		)),
		"Let's setup a new source for your workflow.",
		"A source is a compiled set of OpenAPI specs and overlays that are used as the input for a SDK generation.")).
		Run(); err != nil {
		return nil, err
	}

	promptForDocuments := true
	for promptForDocuments {
		var err error
		sourceDocument, err := promptForDocument("input")
		if err != nil {
			return nil, err
		}

		source.Inputs = append(source.Inputs, *sourceDocument)

		promptForDocuments, err = charm.NewBranchCondition("Would you like to add another input document?")
		if err != nil {
			return nil, err
		}
	}

	promptForOverlays, err := charm.NewBranchCondition("Would you like to add an overlay document?")
	if err != nil {
		return nil, err
	}

	for promptForOverlays {
		var err error
		sourceDocument, err := promptForDocument("overlay")
		if err != nil {
			return nil, err
		}
		source.Overlays = append(source.Overlays, *sourceDocument)

		promptForOverlays, err = charm.NewBranchCondition("Would you like to add another overlay document?")
		if err != nil {
			return nil, err
		}
	}

	totalDocuments := len(source.Inputs) + len(source.Overlays)
	var outputLocation string
	if totalDocuments > 1 {
		if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Provide an output location for your generation target (OPTIONAL):").
					Prompt(" ").
					Inline(true).
					Value(&outputLocation),
			)),
			"You can provide an output location for this built source file.")).
			Run(); err != nil {
			return nil, err
		}
	}

	if outputLocation != "" {
		source.Output = &outputLocation
	}

	// TODO: Attempt to build the source here
	if err := source.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate source")
	}

	inputWorkflow.Sources[sourceName] = *source

	addAnotherSource, err := charm.NewBranchCondition("Would you like to add another source to your workflow file?")
	if err != nil {
		return nil, err
	}

	var nextState State = TargetBase
	if addAnotherSource {
		nextState = SourceBase
	}

	return &nextState, nil
}

func promptForDocument(title string) (*workflow.Document, error) {
	var requiresAuthentication bool
	var fileLocation, authHeader, authSecret string
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("What is the location of your %s document:", title)).
				Placeholder("local file path or remote file reference.").
				Prompt(" ").
				Inline(true).
				Value(&fileLocation),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Does this remote file require authentication:").
				Affirmative("Yes.").
				Negative("No.").
				Value(&requiresAuthentication),
		).WithHideFunc(func() bool {
			_, err := url.ParseRequestURI(fileLocation)
			return err != nil
		}),
		huh.NewGroup(
			huh.NewInput().
				Title("What is the name of your authentication Header:").
				Placeholder("x-auth-token").
				Prompt(" ").
				Inline(true).
				Value(&authHeader),
			huh.NewInput().
				Title("What is the reference to your auth secret:").
				Placeholder("$AUTH_TOKEN").
				Prompt(" ").
				Inline(true).
				Value(&authSecret),
		).WithHideFunc(func() bool {
			return !requiresAuthentication
		}),
	), fmt.Sprintf("Let's add a new %s document to this source.", title))).
		Run(); err != nil {
		return nil, err
	}

	document := &workflow.Document{
		Location: fileLocation,
	}
	if authHeader != "" && authSecret != "" {
		document.Auth = &workflow.Auth{
			Header: authHeader,
			Secret: authSecret,
		}
	}

	if err := document.Validate(); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to validate the provided %s document", title))
	}

	return document, nil
}
