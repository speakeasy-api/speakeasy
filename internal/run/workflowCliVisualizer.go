package run

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/styles"
)

type UpdateMsg string

var (
	MsgUpdated   UpdateMsg = "updated"
	MsgSucceeded UpdateMsg = "succeeded"
	MsgFailed    UpdateMsg = "failed"
)

// A command that waits for the activity on a channel.
func listenForUpdates(sub <-chan UpdateMsg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

type cliVisualizer struct {
	updates  <-chan UpdateMsg // where we'll receive activity notifications
	status   Status
	rootStep *WorkflowStep
	runFn    func() error
	spinner  spinner.Model
	err      error
}

func (m cliVisualizer) Init() tea.Cmd {
	run := func() tea.Msg {
		m.err = m.runFn()
		return tea.Quit
	}

	return tea.Batch(
		listenForUpdates(m.updates),
		run,
		m.spinner.Tick,
	)
}

func (m cliVisualizer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.status = StatusFailed
			return m, tea.Quit
		}
	case UpdateMsg:
		switch msg {
		case MsgUpdated:
			return m, listenForUpdates(m.updates) // wait for next event
		case MsgSucceeded:
			m.status = StatusSucceeded
			return m, tea.Quit
		case MsgFailed:
			m.status = StatusFailed
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m cliVisualizer) View() string {
	statusStyle := styles.Info
	switch m.status {
	case StatusFailed:
		statusStyle = styles.Error
	case StatusSucceeded:
		statusStyle = styles.Success
	}

	summary := m.rootStep.PrettyString()

	if m.status == StatusRunning {
		summary = fmt.Sprintf("%s\n%s", summary, m.spinner.View())
	}

	style := styles.LeftBorder(statusStyle.GetForeground()).
		MarginBottom(2)

	return style.Render(summary)
}

func initSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Meter
	s.Style = lipgloss.NewStyle().Foreground(styles.Colors.Yellow)
	return s
}

func (w *WorkflowStep) RunWithVisualization(runFn func() error, updatesChannel chan UpdateMsg) error {
	v := cliVisualizer{
		updates:  updatesChannel,
		rootStep: w,
		runFn:    runFn,
		spinner:  initSpinner(),
		status:   StatusRunning,
	}
	p := tea.NewProgram(v)

	model, err := p.Run()
	if err != nil {
		return err
	}

	return model.(cliVisualizer).err
}
