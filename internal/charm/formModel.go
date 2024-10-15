package charm

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"slices"
)

type Key struct {
	Key   string
	Label string
}

type FormModel struct {
	title          string
	description    string
	form           *huh.Form // huh.Form is just a tea.Model
	signalExit     bool
	keys           []Key
	disallowedKeys []string
}

func Execute(input huh.Field, opts ...FormOpt) error {
	_, err := NewForm(
		huh.NewForm(
			huh.NewGroup(input),
		),
		opts...,
	).ExecuteForm()
	return err
}

type FormOpt func(*FormModel)

func NewForm(form *huh.Form, opts ...FormOpt) FormModel {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Input.AcceptSuggestion = key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "complete"), key.WithHelp("right", "complete"))

	model := FormModel{
		form: form.WithTheme(formTheme).WithKeyMap(keyMap).WithShowHelp(false),
		keys: []Key{
			{Key: "tab/↵", Label: "next"},
			{Key: "esc", Label: "quit"},
		},
	}

	for _, opt := range opts {
		opt(&model)
	}

	return model
}

func (m FormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.disallowedKeys) > 0 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if slices.Contains(m.disallowedKeys, msg.String()) {
				return m, nil
			}
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

func (m FormModel) HandleKeypress(key string) tea.Cmd {
	return nil
}

// SetWidth Not yet implemented.
func (m FormModel) SetWidth(width int) {}

func (m FormModel) View() string {
	if m.form.State == huh.StateCompleted {
		return ""
	}
	titleStyle := lipgloss.NewStyle().Foreground(styles.Focused.GetForeground()).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(styles.Dimmed.GetForeground()).Italic(true)

	keys := make([]string, len(m.keys))
	labels := make([]string, len(m.keys))
	for i, k := range m.keys {
		keys[i] = k.Key
		labels[i] = k.Label
	}
	legend := styles.RenderKeymapLegend(keys, labels)

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

func (m FormModel) OnUserExit() {}

func (m FormModel) ExecuteForm(opts ...tea.ProgramOption) (tea.Model, error) {
	mResult, err := RunModel(m, opts...)
	if err != nil {
		return mResult, err
	}

	return mResult, nil
}

/*
 * OPTIONS
 */

func WithTitle(title string) FormOpt {
	return func(m *FormModel) {
		m.title = title
	}
}

func WithDescription(description string) FormOpt {
	return func(m *FormModel) {
		m.description = description
	}
}

func WithKey(key, label string) FormOpt {
	return func(m *FormModel) {
		m.keys = append([]Key{{Key: key, Label: label}}, m.keys...)
	}
}

func WithNoSpaces() FormOpt {
	return func(m *FormModel) {
		m.disallowedKeys = append(m.disallowedKeys, " ")
	}
}
