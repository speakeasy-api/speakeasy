package charm

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

func NewBranchPrompt(title string, output *bool) *huh.Group {
	return huh.NewGroup(huh.NewConfirm().
		Title(title).
		Affirmative("Yes.").
		Negative("No.").
		Value(output))
}

func NewSelectPrompt(title string, description string, options []string, output *string) *huh.Group {
	return huh.NewGroup(huh.NewSelect[string]().
		Title(title).
		Description(description + "\n").
		Options(huh.NewOptions(options...)...).
		Value(output))
}

func FormatCommandTitle(title string, description string) string {
	titleStyle := lipgloss.NewStyle().Foreground(styles.Focused.GetForeground()).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(styles.Dimmed.GetForeground()).Italic(true)
	header := titleStyle.Render(title)
	header += "\n" + descriptionStyle.Render(description)
	return header
}

func NewInput() *huh.Input {
	return huh.NewInput().Prompt(" ").Inline(true)
}
