package workflowTracking

import (
	"fmt"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
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
	rootStep *WorkflowStep
	runFn    func() error
	spinner  *spinner.Model
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

func (m cliVisualizer) OnUserExit() {
	m.rootStep.FailWorkflowWithoutNotifying()
	m.rootStep.statusExplanation = "user exited"
}

func (m cliVisualizer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UpdateMsg:
		switch msg {
		case MsgUpdated:
			return m, listenForUpdates(m.updates) // wait for next event
		case MsgSucceeded, MsgFailed:
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	*m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m cliVisualizer) HandleKeypress(key string) tea.Cmd { return nil }
func (m cliVisualizer) SetWidth(width int)                {}

func (m cliVisualizer) View() string {
	statusStyle := styles.Info
	switch m.rootStep.status {
	case StatusFailed:
		statusStyle = styles.Error
	case StatusSucceeded:
		statusStyle = styles.Success
	case StatusSkipped:
		statusStyle = styles.Dimmed
	}

	summary := m.rootStep.PrettyString()

	if m.rootStep.status == StatusRunning {
		summary = fmt.Sprintf("%s\n%s", summary, m.spinner.View())
	}

	style := styles.LeftBorder(statusStyle.GetForeground()).
		MarginBottom(2)

	return style.Render(summary)
}

var _ charm_internal.InternalModel = &cliVisualizer{}

func (w *WorkflowStep) RunWithVisualization(runFn func() error, updatesChannel chan UpdateMsg) error {
	v := cliVisualizer{
		updates:  updatesChannel,
		rootStep: w,
		runFn:    runFn,
		spinner:  interactivity.InitSpinner(),
	}

	_, err := charm_internal.RunModel(&v)

	return err
}
