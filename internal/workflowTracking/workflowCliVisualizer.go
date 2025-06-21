package workflowTracking

import (
	"fmt"
	"strings"
	"sync"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
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

// A command that waits for log messages on a channel.
func listenForLogs(sub <-chan LogMsg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

type LogMsg struct {
	Content string
	Type    log.MsgType
}

// LogChannelAdapter converts log.Msg to LogMsg and sends to a channel
type LogChannelAdapter struct {
	logChannel chan<- LogMsg
}

func NewLogChannelAdapter(ch chan<- LogMsg) *LogChannelAdapter {
	return &LogChannelAdapter{logChannel: ch}
}

func (lca *LogChannelAdapter) SendLogMsg(content string, msgType log.MsgType) {
	select {
	case lca.logChannel <- LogMsg{Content: content, Type: msgType}:
	default:
		// Don't block if channel is full
	}
}

type cliVisualizer struct {
	updates     <-chan UpdateMsg // where we'll receive activity notifications
	logChannel  <-chan LogMsg    // where we'll receive log messages
	rootStep    *WorkflowStep
	runFn       func() error
	spinner     *spinner.Model
	err         error
	logViewport viewport.Model
	logs        []LogMsg
	logsMutex   sync.RWMutex
	width       int
	height      int
}

func (m *cliVisualizer) Init() tea.Cmd {
	run := func() tea.Msg {
		m.err = m.runFn()
		return tea.Quit
	}

	// Initialize viewport with reasonable defaults
	m.logViewport = viewport.New(0, 8) // Default height of 8 lines
	m.logViewport.SetContent("")

	cmds := []tea.Cmd{
		listenForUpdates(m.updates),
		run,
		m.spinner.Tick,
	}

	// Add log listener if channel is available
	if m.logChannel != nil {
		cmds = append(cmds, listenForLogs(m.logChannel))
	}

	return tea.Batch(cmds...)
}

func (m *cliVisualizer) OnUserExit() {
	m.rootStep.FailWorkflowWithoutNotifying()
	m.rootStep.statusExplanation = "user exited"
}

func (m *cliVisualizer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UpdateMsg:
		switch msg {
		case MsgUpdated:
			return m, listenForUpdates(m.updates) // wait for next event
		case MsgSucceeded, MsgFailed:
			return m, tea.Quit
		}
	case LogMsg:
		m.logsMutex.Lock()
		m.logs = append(m.logs, msg)
		
		// Keep only the last 100 log entries to prevent memory issues
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
		
		// Update viewport content
		content := m.formatLogs()
		m.logViewport.SetContent(content)
		m.logViewport.GotoBottom()
		m.logsMutex.Unlock()
		
		// Continue listening for more logs
		return m, listenForLogs(m.logChannel)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()
		return m, nil
	}

	var cmd tea.Cmd
	*m.spinner, cmd = m.spinner.Update(msg)
	m.logViewport, _ = m.logViewport.Update(msg)
	return m, cmd
}

func (m *cliVisualizer) HandleKeypress(key string) tea.Cmd { return nil }
func (m *cliVisualizer) SetWidth(width int)                {
	m.width = width
	m.updateViewportSize()
}

func (m *cliVisualizer) updateViewportSize() {
	if m.width > 0 {
		// Set viewport width to terminal width with some padding
		m.logViewport.Width = m.width - 4
		
		// Dynamic height based on terminal size, but keep it reasonable
		logHeight := 8
		if m.height > 20 {
			logHeight = 12
		}
		m.logViewport.Height = logHeight
	}
}

func (m *cliVisualizer) formatLogs() string {
	m.logsMutex.RLock()
	defer m.logsMutex.RUnlock()
	
	var formatted strings.Builder
	for _, logMsg := range m.logs {
		// Apply styling based on log type
		switch logMsg.Type {
		case log.MsgError:
			formatted.WriteString(styles.Error.Render(logMsg.Content))
		case log.MsgWarn:
			formatted.WriteString(styles.Warning.Render(logMsg.Content))
		case log.MsgInfo:
			formatted.WriteString(styles.Info.Render(logMsg.Content))
		default:
			formatted.WriteString(logMsg.Content)
		}
		formatted.WriteString("\n")
	}
	return strings.TrimSuffix(formatted.String(), "\n")
}

func (m *cliVisualizer) View() string {
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

	workflowStyle := styles.LeftBorder(statusStyle.GetForeground()).
		MarginBottom(1)
	workflowSection := workflowStyle.Render(summary)

	// Create log section
	logTitle := styles.DimmedItalic.Render("Logs:")
	logBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
	
	logContent := m.logViewport.View()
	if strings.TrimSpace(logContent) == "" {
		logContent = styles.DimmedItalic.Render("No logs yet...")
	}
	
	logBox := logBorder.Render(logContent)
	
	// Combine workflow and log sections
	return lipgloss.JoinVertical(lipgloss.Left, workflowSection, logTitle, logBox)
}

var _ charm_internal.InternalModel = (*cliVisualizer)(nil)

func (w *WorkflowStep) RunWithVisualization(runFn func() error, updatesChannel chan UpdateMsg) error {
	return w.RunWithVisualizationAndLogs(runFn, updatesChannel, nil)
}

func (w *WorkflowStep) RunWithVisualizationAndLogs(runFn func() error, updatesChannel chan UpdateMsg, logChannel <-chan LogMsg) error {
	v := &cliVisualizer{
		updates:    updatesChannel,
		logChannel: logChannel,
		rootStep:   w,
		runFn:      runFn,
		spinner:    interactivity.InitSpinner(),
		logs:       make([]LogMsg, 0),
	}

	_, err := charm_internal.RunModel(v)

	return err
}
