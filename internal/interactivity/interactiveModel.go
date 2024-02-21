package interactivity

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

type InteractiveModel interface {
	Init()
	HandleKeypress(key string) bool // return true to signal exit
	Render() string
	SetWidth(width int)
}

type interactiveModelInternal struct {
	model      InteractiveModel
	signalExit bool
}

func (m interactiveModelInternal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.signalExit = true
			return m, tea.Quit
		default:
			if m.model.HandleKeypress(keypress) {
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.model.SetWidth(msg.Width)
	}

	return m, nil
}

func (m interactiveModelInternal) View() string {
	return m.model.Render()
}

func (m interactiveModelInternal) Init() tea.Cmd {
	m.model.Init()
	return nil
}

func RunInteractiveModel(m InteractiveModel) InteractiveModel {
	model := interactiveModelInternal{
		model: m,
	}
	if mResult, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	} else {
		if m, ok := mResult.(interactiveModelInternal); ok {
			if m.signalExit {
				os.Exit(0)
			}

			return m.model
		}
	}

	return nil
}

var _ tea.Model = interactiveModelInternal{}
