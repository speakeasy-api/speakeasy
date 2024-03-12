package interactivity

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

type SimpleInput struct {
	input      InputField
	inputModel textinput.Model
	validate   func(s string) error

	cursorMode cursor.Mode
	focusIndex int
	done       bool
}

func NewSimpleInput(input InputField, validate func(s string) error) SimpleInput {
	m := SimpleInput{
		input:    input,
		done:     false,
		validate: validate,
	}

	t := textinput.New()
	t.Prompt = input.Name + ": "
	t.Placeholder = input.Placeholder
	t.SetValue(input.Value)

	t.Focus()
	t.PromptStyle = focusedPromptStyle
	t.TextStyle = styles.Focused
	t.Cursor.Style = styles.Cursor
	t.SetSuggestions(charm_internal.SchemaFilesInCurrentDir(""))

	m.inputModel = t

	return m
}

func (m *SimpleInput) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SimpleInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle character input and blinking
	i, cmd := m.inputModel.Update(msg)
	m.inputModel = i

	return m, cmd
}

func (m *SimpleInput) HandleKeypress(key string) tea.Cmd {
	switch key {
	case "enter":
		if m.Validate() == nil {
			m.done = true
			return tea.Quit
		} else {
			break
		}
	default:
		if suggestions := charm_internal.SuggestionCallback(m.inputModel.Value()); len(suggestions) > 0 {
			m.inputModel.SetSuggestions(suggestions)
		}
	}

	return nil
}

// SetWidth Not yet implemented.
func (m *SimpleInput) SetWidth(width int) {}

func (m *SimpleInput) Validate() error {
	if m.inputModel.Value() == "" {
		return fmt.Errorf("please supply a value")
	}
	return m.validate(m.inputModel.Value())
}

func (m *SimpleInput) View() string {
	if m.done {
		successMessage := fmt.Sprintf("✔ %s", m.inputModel.Value())
		return styles.Success.Copy().
			Margin(0, 2, 1, 2).
			Render(successMessage)
	}

	input := m.inputModel.View()
	helper := styles.Success.Render("✔")

	if err := m.Validate(); err != nil {
		helper = styles.Error.Render(fmt.Sprintf("✖ %s", err))
	}

	return inputBoxStyle.Render(fmt.Sprintf("%s\n%s", input, helper))
}

// Run returns a map from input name to the input value
func (m *SimpleInput) Run() string {
	newM, err := charm_internal.RunModel(m)
	if err != nil {
		os.Exit(1)
	}

	resultingModel := newM.(*SimpleInput)

	return resultingModel.inputModel.Value()
}
