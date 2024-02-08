package charm

import (
	"fmt"

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

func NewSelectPrompt(title string, description string, options []huh.Option[string], output *string) *huh.Group {
	group := huh.NewGroup(huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(output))

	if description != "" {
		group = group.Description(description + "\n")
	}

	return group
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

func FormatEditOption(text string) string {
	return fmt.Sprintf("✎ %s", text)
}

func FormatNewOption(text string) string {
	return fmt.Sprintf("+ %s", text)
}
