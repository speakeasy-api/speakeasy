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
	titleStyle := lipgloss.NewStyle().Foreground(Focused.GetForeground()).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(Dimmed.GetForeground()).Italic(true)
	header := titleStyle.Render(title)
	header += "\n" + descriptionStyle.Render(description)
	return header
}

func NewInput() *huh.Input {
	return huh.NewInput().Prompt(" ").Inline(true)
}
