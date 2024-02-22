package charm

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

type FormModel struct {
	title       string
	description string
	form        *huh.Form // huh.Form is just a tea.Model
}

func NewForm(form *huh.Form, args ...string) FormModel {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Input.AcceptSuggestion = key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "complete"), key.WithHelp("right", "complete"))
	model := FormModel{
		form: form.WithTheme(formTheme).WithKeyMap(keyMap).WithShowHelp(false),
	}

	if len(args) > 0 {
		model.title = args[0]
		if len(args) > 1 {
			model.description = args[1]
		}
	}

	return model
}

func (m FormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	// Quit when the form is done.
	if m.form.State == huh.StateCompleted {
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
}

// SetWidth Not yet implemented.
func (m FormModel) SetWidth(width int) {}

func (m FormModel) View() string {
	if m.form.State == huh.StateCompleted {
		return ""
	}
	titleStyle := lipgloss.NewStyle().Foreground(styles.Focused.GetForeground()).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(styles.Dimmed.GetForeground()).Italic(true)

	legend := styles.KeymapLegend([]string{"tab/↵", "esc"}, []string{"next", "quit"})
	content := m.form.View() + "\n" + legend + "\n"

	if m.title != "" {
		header := titleStyle.Render(m.title)
		if m.description != "" {
			header += "\n" + descriptionStyle.Render(m.description)
		}
		return header + "\n\n" + content
	}

	return content
}

func (m FormModel) ExecuteForm(opts ...tea.ProgramOption) (tea.Model, error) {
	mResult, err := RunModel(m, opts...)
	if err != nil {
		return mResult, err
	}

	return mResult, nil
}
