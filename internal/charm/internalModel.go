package charm

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type InternalModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
}

type internalModel struct {
	model InternalModel
}

func (m internalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	fmt.Println("ENTERING UPDATE")
	return m.model.Update(msg)
}

func (m internalModel) View() string {
	fmt.Println("ENTERING VIEW")
	return m.model.View()
}

func (m internalModel) Init() tea.Cmd {
	return m.model.Init()
}

func RunModel(m InternalModel, opts ...tea.ProgramOption) (tea.Model, error) {
	fmt.Println("ENTERING RUN MODEL")
	model := internalModel{
		model: m,
	}
	if mResult, err := tea.NewProgram(model, opts...).Run(); err != nil {
		return mResult, err
	} else {
		if _, ok := mResult.(internalModel); ok {
			fmt.Println("MODEL IS INTERNAL MODEL")
			return mResult, nil
		}
		fmt.Println("DID NOT MATCH INTERNAL MODEL")
		return mResult, err
	}
}

var _ tea.Model = internalModel{}
