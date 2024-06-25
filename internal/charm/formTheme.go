package charm

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

var formTheme *huh.Theme

func init() {
	t := *huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(styles.Focused.GetForeground())
	t.Focused.Title = t.Focused.Title.Foreground(styles.Focused.GetForeground()).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(styles.Dimmed.GetForeground()).Italic(true).Inline(false)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(styles.Colors.Red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(styles.Colors.Red)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(styles.Focused.GetForeground()).Bold(true)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(styles.Focused.GetForeground()).Bold(true)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(styles.Focused.GetForeground()).Bold(true)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(styles.Focused.GetForeground()).SetString("✓ ").Bold(true)
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().SetString("")
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(styles.Colors.Green)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Background(styles.Dimmed.GetForeground())
	t.Focused.Next = t.Focused.FocusedButton

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(styles.Colors.WhiteBlackAdaptive)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(styles.Dimmed.GetForeground()).Italic(true)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(styles.Colors.WhiteBlackAdaptive)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(styles.Colors.WhiteBlackAdaptive)

	t.Blurred.Description = t.Focused.Description
	t.Blurred.TextInput.Placeholder = t.Blurred.TextInput.Placeholder.Italic(true)
	t.Blurred.Title = t.Blurred.Title.Foreground(styles.FocusedDimmed.GetForeground())
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(styles.FocusedDimmed.GetForeground())
	t.Blurred.SelectedOption = t.Blurred.SelectedOption.Foreground(styles.FocusedDimmed.GetForeground())
	t.Blurred.SelectSelector = t.Blurred.SelectSelector.Foreground(styles.FocusedDimmed.GetForeground())
	t.Blurred.SelectedPrefix = lipgloss.NewStyle().Foreground(styles.FocusedDimmed.GetForeground()).SetString("✓ ")
	t.Blurred.UnselectedPrefix = lipgloss.NewStyle().SetString("")

	formTheme = &t
}
