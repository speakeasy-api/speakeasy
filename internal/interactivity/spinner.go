package interactivity

// A simple program demonstrating the spinner component from the Bubbles
// component library.

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

type model struct {
	message string
	spinner spinner.Model
	quit    bool
}

func initialModel(message string) *model {
	return &model{
		message: message,
		spinner: *InitSpinner(),
	}
}

func InitSpinner() *spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Meter
	s.Style = lipgloss.NewStyle().Foreground(styles.Colors.Yellow)
	return &s
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.QuitMsg); ok {
		m.quit = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if m.quit {
		return ""
	}

	message := styles.HeavilyEmphasized.Render(m.message)
	messageWidth := lipgloss.Width(message)

	// Pad spinner to message width so the box size remains consistent during animation
	spinnerView := lipgloss.NewStyle().Width(messageWidth).Align(lipgloss.Center).Render(m.spinner.View())

	s := fmt.Sprintf("%s\n%s", message, spinnerView)
	// Add trailing newline to prevent the bottom border from being cut off during animation
	return styles.MakeBoxed(s, styles.Colors.DimYellow, lipgloss.Center) + "\n"
}

func StartSpinner(message string) func() {
	p := tea.NewProgram(initialModel(message))
	go func() {
		_, err := p.Run()
		if err != nil {
			println(err.Error())
		}
	}()

	return func() {
		p.Quit()
		// Very important, otherwise the TUI will be borked and future logs will be messed up
		_ = p.ReleaseTerminal()
	}
}
