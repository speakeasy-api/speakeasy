package run

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"os"
	"strings"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusSucceeded Status = "success"
)

type WorkflowStep struct {
	name     string
	status   Status
	substeps []*WorkflowStep
	nextStep *WorkflowStep
	updates  chan<- UpdateMsg
}

func NewWorkflowStep(name string, sub chan<- UpdateMsg) *WorkflowStep {
	return &WorkflowStep{
		name:     name,
		status:   StatusRunning,
		substeps: []*WorkflowStep{},
		updates:  sub,
	}
}

func (w *WorkflowStep) SetNextStep(next *WorkflowStep) {
	w.status = StatusSucceeded // If we go to the next step, we're successful
	w.nextStep = next

	w.Notify()
}

func (w *WorkflowStep) NextStep(name string) *WorkflowStep {
	next := NewWorkflowStep(name, w.updates)

	w.status = StatusSucceeded // If we go to the next step, we're successful
	w.nextStep = next

	w.Notify()

	return next
}

func (w *WorkflowStep) NextSubstep(name string) *WorkflowStep {
	substep := NewWorkflowStep(name, w.updates)

	w.AddSubstep(substep)

	return substep
}

func (w *WorkflowStep) AddSubstep(substep *WorkflowStep) {
	if len(w.substeps) > 0 {
		w.substeps[len(w.substeps)-1].status = StatusSucceeded // If we go to the next substep, we're successful
	}
	w.substeps = append(w.substeps, substep)

	w.Notify()
}

func (w *WorkflowStep) SucceedWorkflow() {
	if w.status != StatusFailed {
		w.status = StatusSucceeded
	}
	for _, substep := range w.substeps {
		substep.SucceedWorkflow()
	}
	if w.nextStep != nil {
		w.nextStep.SucceedWorkflow()
	}

	w.Notify()
}

func (w *WorkflowStep) FailWorkflow() {
	if w.status != StatusSucceeded {
		w.status = StatusFailed
	}
	for _, substep := range w.substeps {
		substep.FailWorkflow()
	}
	if w.nextStep != nil {
		w.nextStep.FailWorkflow()
	}

	w.Notify()
}

func (w *WorkflowStep) Notify() {
	if w.updates != nil {
		w.updates <- MsgUpdated
	}
}

func (w *WorkflowStep) PrettyString() string {
	return w.toString(0, 0)
}

func (w *WorkflowStep) toString(parentIndent, indent int) string {
	builder := &strings.Builder{}

	indentString := ""
	if indent > 0 {
		terminator := "└─"
		if indent == parentIndent {
			terminator = "  "
		}
		indentString = strings.Repeat("  ", indent-1) + terminator
	}

	s := fmt.Sprintf("%s%s", indentString, w.name)

	style := styles.Info
	switch w.status {
	case StatusFailed:
		style = styles.Error
	case StatusRunning:
		style = styles.Info
	case StatusSucceeded:
		style = styles.Success
	}

	statusStyle := style.Copy().Bold(false).Italic(true)

	builder.WriteString(style.Render(s))
	builder.WriteString(statusStyle.Render(" -", string(w.status)))

	for _, child := range w.substeps {
		builder.WriteString("\n")
		builder.WriteString(child.toString(indent, indent+1))
	}

	if w.nextStep != nil {
		builder.WriteString("\n")
		builder.WriteString(w.nextStep.toString(indent, indent))
	}

	return builder.String()
}

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
}

func (m cliVisualizer) Init() tea.Cmd {
	run := func() tea.Msg {
		err := m.runFn()
		if err != nil {
			println("GOt error")
		}
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

	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true). // Left border only
		BorderForeground(statusStyle.GetForeground()).
		PaddingLeft(1).
		MarginBottom(2)

	return style.Render(summary)
}

func initSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Meter
	s.Style = lipgloss.NewStyle().Foreground(styles.Colors.Yellow)
	return s
}

func (w *WorkflowStep) RunWithVisualization(runFn func() error, updatesChannel chan UpdateMsg) {
	p := tea.NewProgram(cliVisualizer{
		updates:  updatesChannel,
		rootStep: w,
		runFn:    runFn,
		spinner:  initSpinner(),
		status:   StatusRunning,
	})

	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
		os.Exit(1)
	}
}
