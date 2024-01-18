package charm

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func NewBranchPrompt(title string, output *bool) *huh.Group {
	return huh.NewGroup(huh.NewConfirm().
		Title(title).
		Affirmative("Yes.").
		Negative("No.").
		Value(output))
}

func FormatCommandTitle(title string, description string) string {
	titleStyle := lipgloss.NewStyle().Foreground(yellow).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(grey).Italic(true).Bold(true)
	header := titleStyle.Render(title)
	header += "\n" + descriptionStyle.Render(description)
	return header
}
