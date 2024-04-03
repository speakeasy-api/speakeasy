package interactivity

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

var (
	titleStyle       = styles.HeavilyEmphasized.Copy().MarginLeft(2)
	descriptionStyle = styles.Dimmed.Copy().MarginLeft(2).Foreground(styles.Colors.BrightGrey)

	inputBoxStyle = styles.Margins.Copy().
			PaddingLeft(2).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(styles.Colors.DimYellow)

	focusedPromptStyle = styles.Focused.Copy().Bold(true)
	blurredPromptStyle = focusedPromptStyle.Copy().Foreground(styles.Colors.WhiteBlackAdaptive)
	placeholderStyle   = styles.Dimmed.Copy()
)

type MultiInput struct {
	title       string
	description string
	inputs      []InputField

	inputModels    []textinput.Model
	inputsRequired bool

	cursorMode cursor.Mode
	focusIndex int
	done       bool
}

type InputField struct {
	Name                       string
	Placeholder                string
	Value                      string
	AutocompleteFileExtensions []string
}

func NewMultiInput(title, description string, required bool, inputs ...InputField) MultiInput {
	m := MultiInput{
		title:          title,
		description:    description,
		inputModels:    make([]textinput.Model, len(inputs)),
		inputs:         inputs,
		inputsRequired: required,
		done:           false,
	}

	var t textinput.Model
	for i, input := range inputs {
		t = textinput.New()

		t.Prompt = input.Name
		t.Placeholder = input.Placeholder
		t.SetValue(input.Value)
		t.Cursor.Style = styles.Cursor
		if len(input.AutocompleteFileExtensions) > 0 {
			suggestions := charm_internal.SchemaFilesInCurrentDir("", input.AutocompleteFileExtensions)
			t.SetSuggestions(suggestions)
			t.ShowSuggestions = len(suggestions) > 0
			t.KeyMap.AcceptSuggestion.SetEnabled(len(suggestions) > 0)
		}

		m.inputModels[i] = t
	}

	// Focus will initialize the necessary styles
	m.Focus(0)

	return m
}

func (m *MultiInput) Init() tea.Cmd {
	return textinput.Blink
}

func (m *MultiInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *MultiInput) HandleKeypress(key string) tea.Cmd {
	switch key {
	// Set focus to next input
	case "shift+tab", "enter", "up", "down":
		// Did the user press enter while the submit button was focused?
		// If so, exit.
		if key == "enter" && m.focusIndex == len(m.inputModels) {
			if m.Validate() {
				m.done = true
				return tea.Quit
			} else {
				break
			}
		}

		// Cycle indexes
		if key == "up" || key == "shift+tab" {
			m.focusIndex--
		} else {
			m.focusIndex++
		}

		if m.focusIndex > len(m.inputModels) {
			m.focusIndex = 0
		} else if m.focusIndex < 0 {
			m.focusIndex = len(m.inputModels)
		}

		return m.Focus(m.focusIndex)
	default:
		if len(m.inputs[m.focusIndex].AutocompleteFileExtensions) > 0 {
			if suggestions := charm_internal.SuggestionCallback(charm_internal.SuggestionCallbackConfig{
				FileExtensions: m.inputs[m.focusIndex].AutocompleteFileExtensions,
			})(m.inputModels[m.focusIndex].Value()); len(suggestions) > 0 {
				m.inputModels[m.focusIndex].ShowSuggestions = true
				m.inputModels[m.focusIndex].KeyMap.AcceptSuggestion.SetEnabled(true)
				m.inputModels[m.focusIndex].SetSuggestions(suggestions)
			}
		}
	}

	return nil
}

// SetWidth Not yet implemented.
func (m *MultiInput) SetWidth(width int) {}

func (m *MultiInput) Focus(index int) tea.Cmd {
	var cmd tea.Cmd

	for i := 0; i <= len(m.inputModels)-1; i++ {
		if i == index {
			cmd = m.inputModels[i].Focus()
			m.inputModels[i].PromptStyle = focusedPromptStyle
			m.inputModels[i].TextStyle = styles.Focused
			continue
		}

		m.inputModels[i].Blur()
		m.inputModels[i].PromptStyle = blurredPromptStyle
		m.inputModels[i].PlaceholderStyle = placeholderStyle
		m.inputModels[i].TextStyle = styles.None

	}

	return cmd
}

func (m *MultiInput) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputModels))

	// Only text inputModels with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputModels {
		m.inputModels[i], cmds[i] = m.inputModels[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m *MultiInput) Validate() bool {
	if !m.inputsRequired {
		return true
	}

	valid := true
	for i := range m.inputModels {
		valid = valid && m.inputModels[i].Value() != ""
	}

	return valid
}

func (m *MultiInput) View() string {
	if m.done {
		fieldsString := "fields have"
		if len(m.getFilledValues()) == 1 {
			fieldsString = "field has"
		}
		successMessage := fmt.Sprintf("Values for %d %s been supplied ✔\n", len(m.getFilledValues()), fieldsString)
		return styles.Success.Copy().
			Margin(0, 2, 1, 2).
			Render(successMessage)
	}

	var inputsView strings.Builder

	for _, inputModel := range m.inputModels {
		inputModel.Prompt = inputModel.Prompt + ": " // Add this here so its only rendered, not actually set as the prompt
		inputsView.WriteString(inputModel.View())
		inputsView.WriteString("\n\n")
	}

	valid := m.Validate()

	helperText := ""
	if !valid {
		helperText = "All fields are required"
	}

	button := Button{
		Label:    "Continue",
		Disabled: !valid,
		Hovered:  m.focusIndex == len(m.inputModels),
	}

	buttonString := button.View()

	if m.inputsRequired {
		buttonString = ButtonWithHelperText{
			Button:          button,
			HelperText:      helperText,
			ShowOnlyOnHover: true,
		}.View()
	}

	inputsView.WriteString(buttonString)

	inputsString := inputBoxStyle.Render(inputsView.String())

	titleString := titleStyle.Render(m.title)
	descriptionString := descriptionStyle.Render(m.description)

	return fmt.Sprintf("%s\n%s\n%s", titleString, descriptionString, inputsString)
}

func (m *MultiInput) getFilledValues() map[string]string {
	inputResults := make(map[string]string)
	for _, input := range m.inputModels {
		if input.Value() != "" {
			inputResults[input.Prompt] = input.Value()
		}
	}

	return inputResults
}

func (m *MultiInput) OnUserExit() {}

// Run returns a map from input name to the input value
func (m *MultiInput) Run() map[string]string {
	newM, err := charm_internal.RunModel(m)
	if err != nil {
		os.Exit(1)
	}

	resultingModel := newM.(*MultiInput)

	return resultingModel.getFilledValues()
}
