package quickstart

import (
	"fmt"
	"net/url"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/speakeasy-api/sdk-gen-config/workflow"
)

func sourceBaseForm(inputWorkflow *workflow.Workflow) (*State, error) {
	source := &workflow.Source{}
	var sourceName, outputLocation string
	if err := huh.NewForm(
		huh.NewGroup(
			// TODO: Wrap forms into a custom model and restyle the overall title so it's not a note
			huh.NewNote().
				Title("Setup a source for your workflow."),
			huh.NewInput().
				Title("What is a good name for this source?").
				Validate(func(s string) error {
					if _, ok := inputWorkflow.Sources[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(&sourceName),
			huh.NewInput().
				Title("Provide an output location for your built source file (OPTIONAL).").
				Value(&outputLocation),
		)).WithTheme(theme).
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

		promptForDocuments, err = newBranchCondition("Would you like to add another input document?")
		if err != nil {
			return nil, err
		}
	}

	promptForOverlays, err := newBranchCondition("Would you like to add an overlay document?")
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

		promptForOverlays, err = newBranchCondition("Would you like to add another overlay document?")
		if err != nil {
			return nil, err
		}
	}

	if outputLocation != "" {
		source.Output = &outputLocation
	}

	// TODO: Should we also attempt to build this source here?
	if err := source.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate source")
	}

	inputWorkflow.Sources[sourceName] = *source

	addAnotherSource, err := newBranchCondition("Would you like to add another source to your workflow file?")
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
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("Add a new %s document to this source.", title)),
			huh.NewInput().
				Title(fmt.Sprintf("What is the location of your %s document This can be a local path or remote file reference.", title)).
				Value(&fileLocation),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Does this remote file require authentication?").
				Affirmative("Yes.").
				Negative("No.").
				Value(&requiresAuthentication),
		).WithHideFunc(func() bool {
			_, err := url.ParseRequestURI(fileLocation)
			return err != nil
		}),
		huh.NewGroup(
			huh.NewInput().
				Title("What is the name of your authentication Header?").
				Placeholder("x-auth-token").
				Value(&authHeader),
			huh.NewInput().
				Title("What is the reference to your auth secret?").
				Placeholder("$AUTH_TOKEN").
				Value(&authSecret),
		).WithHideFunc(func() bool {
			return !requiresAuthentication
		}),
	).WithTheme(theme).
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
