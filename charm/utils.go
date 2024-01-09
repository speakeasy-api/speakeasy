package charm

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func NewBranchCondition(title string) (bool, error) {
	var value bool
	if _, err := tea.NewProgram(NewForm(huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title(title).
		Affirmative("Yes.").
		Negative("No.").
		Value(&value))).WithTheme(theme))).Run(); err != nil {
		return false, err
	}

	return value, nil
}

func FormatCommandTitle(title string, description string) string {
	titleStyle := lipgloss.NewStyle().Foreground(yellow).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(grey).Italic(true).Bold(true)
	header := titleStyle.Render(title)
	header += "\n" + descriptionStyle.Render(description)
	return header
}
