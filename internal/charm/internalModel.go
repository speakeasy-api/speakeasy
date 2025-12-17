package charm

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type InternalModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	HandleKeypress(key string) tea.Cmd // A convenience method for handling keypresses. Should usually return nil.
	View() string
	SetWidth(width int)
	SetHeight(height int)
	OnUserExit()
}

type modelWrapper struct {
	model      InternalModel
	signalExit bool
}

func (m modelWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.model.OnUserExit()
			m.signalExit = true
			return m, tea.Quit
		case "esc":
			// Check if the model wants to handle esc (e.g., for form cancellation)
			if m.model.HandleKeypress(keypress) != nil {
				// Model wants to handle it - pass through to Update
				break
			}
			// Model didn't handle it, treat as exit
			m.model.OnUserExit()
			m.signalExit = true
			return m, tea.Quit
		default:
			if cmd := m.model.HandleKeypress(keypress); cmd != nil {
				return m, cmd
			}
		}
	case tea.WindowSizeMsg:
		m.model.SetWidth(msg.Width)
		m.model.SetHeight(msg.Height)
	}

	// Capture the updated model to preserve state changes
	newModel, cmd := m.model.Update(msg)
	if im, ok := newModel.(InternalModel); ok {
		m.model = im
	}
	return m, cmd
}

func (m modelWrapper) View() string {
	return m.model.View()
}

func (m modelWrapper) Init() tea.Cmd {
	return m.model.Init()
}

func RunModel(m InternalModel, opts ...tea.ProgramOption) (InternalModel, error) {
	model := modelWrapper{
		model: m,
	}
	if mResult, err := tea.NewProgram(model, opts...).Run(); err != nil {
		return nil, err
	} else {
		if m, ok := mResult.(modelWrapper); ok {
			if m.signalExit {
				os.Exit(0)
			}

			return m.model, nil
		}

		fmt.Println(mResult)
	}

	return nil, nil
}

var _ tea.Model = modelWrapper{}
