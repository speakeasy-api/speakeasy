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

func sourceBaseForm(quickstart *Quickstart) (*State, error) {
	source := &workflow.Source{}
	var sourceName, fileLocation, authHeader, authSecret string
	var requiresAuthentication bool
	if len(quickstart.WorkflowFile.Sources) == 0 {
		sourceName = "my-first-source"
	}
	if _, err := tea.NewProgram(charm.NewForm(huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("A name for this source:").
				Prompt(" ").
				Inline(true).
				Validate(func(s string) error {
					if _, ok := quickstart.WorkflowFile.Sources[s]; ok {
						return fmt.Errorf("a source with the name %s already exists", s)
					}
					return nil
				}).
				Value(&sourceName),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("What is the location of your OpenAPI document:").
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
		})),
		"Let's setup a new source for your workflow.",
		"A source is a compiled set of OpenAPI specs and overlays that are used as the input for a SDK generation.")).
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

	source.Inputs = append(source.Inputs, *document)

	if err := source.Validate(); err != nil {
		return nil, errors.Wrap(err, "failed to validate source")
	}

	quickstart.WorkflowFile.Sources[sourceName] = *source

	nextState := TargetBase

	return &nextState, nil
}
