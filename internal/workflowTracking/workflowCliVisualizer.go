package workflowTracking

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/huh"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
)

// UpdateMsg is the interface for all messages sent through the updates channel.
// This allows both status updates and interactive prompt requests.
type UpdateMsg interface {
	isUpdateMsg()
}

// StatusMsg represents workflow status updates (updated, succeeded, failed).
type StatusMsg string

func (StatusMsg) isUpdateMsg() {}

const (
	MsgUpdated   StatusMsg = "updated"
	MsgSucceeded StatusMsg = "succeeded"
	MsgFailed    StatusMsg = "failed"
)

// PromptRequestMsg requests the visualizer to pause and run an interactive prompt.
// The worker goroutine blocks on RespCh until the prompt completes.
type PromptRequestMsg struct {
	Form   *huh.Form
	RespCh chan error // nil error means form completed successfully
}

func (PromptRequestMsg) isUpdateMsg() {}

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

	// Prompt state - when non-nil, visualizer is showing a form
	promptForm   *huh.Form
	promptRespCh chan error
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
	// If we're showing a prompt form, delegate all messages to it
	if m.promptForm != nil {
		form, cmd := m.promptForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.promptForm = f

			// Check if form is completed
			if m.promptForm.State == huh.StateCompleted || m.promptForm.State == huh.StateAborted {
				var err error
				if m.promptForm.State == huh.StateAborted {
					err = huh.ErrUserAborted
				}
				// Send response to the waiting goroutine
				if m.promptRespCh != nil {
					m.promptRespCh <- err
				}
				// Clear prompt state and resume normal operation
				m.promptForm = nil
				m.promptRespCh = nil
				// Restart both the update listener AND the spinner tick
				return m, tea.Batch(listenForUpdates(m.updates), m.spinner.Tick)
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case StatusMsg:
		switch msg {
		case MsgUpdated:
			return m, listenForUpdates(m.updates) // wait for next event
		case MsgSucceeded, MsgFailed:
			return m, tea.Quit
		}
	case PromptRequestMsg:
		// Switch to prompt mode - show the form instead of the workflow
		m.promptForm = msg.Form
		m.promptRespCh = msg.RespCh
		// Initialize the form
		return m, m.promptForm.Init()
	}

	var cmd tea.Cmd
	*m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m cliVisualizer) HandleKeypress(key string) tea.Cmd {
	// When showing a form, signal that we want to handle esc for form cancellation
	// The actual handling happens in Update -> form.Update
	if m.promptForm != nil && key == "esc" {
		return func() tea.Msg { return nil } // Non-nil cmd signals we handle it
	}
	return nil
}
func (m cliVisualizer) SetWidth(width int)   {}
func (m cliVisualizer) SetHeight(height int) {}

func (m cliVisualizer) View() string {
	// If showing a prompt form, render that instead
	if m.promptForm != nil {
		return m.promptForm.View()
	}

	statusStyle := styles.Info
	switch m.rootStep.status {
	case StatusRunning:
		statusStyle = styles.Info
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
