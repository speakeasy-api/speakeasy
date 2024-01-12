package charm

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	title       string
	description string
	form        *huh.Form // huh.Form is just a tea.Model
}

func NewForm(form *huh.Form, args ...string) Model {
	model := Model{
		form: form.WithTheme(theme),
	}

	if len(args) > 0 {
		model.title = args[0]
		if len(args) > 1 {
			model.description = args[1]
		}
	}

	return model
}

func (m Model) Init() tea.Cmd {
	return m.form.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			return m, tea.Quit
		}
	}

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

func (m Model) View() string {
	if m.form.State == huh.StateCompleted {
		return ""
	}
	titleStyle := lipgloss.NewStyle().Foreground(yellow).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(grey).Italic(true).Bold(true)
	if m.title != "" {
		header := titleStyle.Render(m.title)
		if m.description != "" {
			header += "\n" + descriptionStyle.Render(m.description)
		}
		return header + "\n\n" + m.form.View()
	}

	return m.form.View()
}
