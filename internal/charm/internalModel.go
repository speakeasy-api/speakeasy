package charm

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type InternalModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
	SetWidth(width int)
}

type internalModel struct {
	model      InternalModel
	signalExit bool
}

func (m internalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.signalExit = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.model.SetWidth(msg.Width)
	}

	_, cmd := m.model.Update(msg)
	return m, cmd
}

func (m internalModel) View() string {
	return m.model.View()
}

func (m internalModel) Init() tea.Cmd {
	return m.model.Init()
}

func RunModel(m InternalModel, opts ...tea.ProgramOption) (InternalModel, error) {
	model := internalModel{
		model: m,
	}
	if mResult, err := tea.NewProgram(model, opts...).Run(); err != nil {
		return nil, err
	} else {
		if m, ok := mResult.(internalModel); ok {
			if m.signalExit {
				os.Exit(0)
			}

			return m.model, nil
		}

		fmt.Println(mResult)
	}

	return nil, nil
}

var _ tea.Model = internalModel{}
