package charmtest

import (
	"io"
	"testing"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/speakeasy-api/huh"
)

// Model is a test model for Charm/TUI testing.
type Model struct {
	// Underlying teatest model.
	*teatest.TestModel

	// If the model is based on huh.Input, huh.Group, or huh.Form, access to the
	// original form can be useful for post submission value assertions.
	Form *huh.Form
}

// Creates a test model from huh.Group(s) for testing purposes.
func ModelFromHuhGroup(t *testing.T, groups ...*huh.Group) *Model {
	t.Helper()

	form := huh.NewForm(groups...)
	teaModelOpts := teatest.WithInitialTermSize(120, 100)
	teaModel := teatest.NewTestModel(t, form, teaModelOpts)

	return &Model{
		Form:      form,
		TestModel: teaModel,
	}
}

// LogOutput logs the current output buffer of the model. This is useful for
// debugging purposes, however it also consumes the output buffer, so this is
// should only be used for debugging.
func (m Model) LogOutput(t *testing.T) {
	t.Helper()

	output, err := io.ReadAll(m.TestModel.Output())

	if err != nil {
		t.Fatalf("error reading output: %s", err)
	}

	t.Logf("Current model output buffer:\n%s", output)
}

// Quits the model to finalize the testing.
func (m Model) Quit(t *testing.T) {
	t.Helper()

	if err := m.TestModel.Quit(); err != nil {
		t.Fatalf("error quitting: %s", err)
	}

	m.TestModel.WaitFinished(t)
}
