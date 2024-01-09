package quickstart

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Temporary file for themes before merging with interactive mode work. This wouldn't live under quickstart
var theme *huh.Theme

func init() {
	theme = huh.ThemeCharm()
	theme.Form = theme.Form.Copy()
	theme.FieldSeparator = theme.FieldSeparator.Copy()
	theme.FieldSeparator.Foreground(lipgloss.Color("#FBE331"))
	theme.FieldSeparator.Background(lipgloss.Color("#FBE331"))
	theme.Form = theme.Group.Copy()
	theme.Form.Foreground(lipgloss.Color("#FBE331"))
	theme.Form.Background(lipgloss.Color("#FBE331"))
	theme.Group.Foreground(lipgloss.Color("#FBE331"))
	theme.Group.Background(lipgloss.Color("#FBE331"))
	f := &theme.Focused
	b := &theme.Blurred
	f.Title.Foreground(lipgloss.AdaptiveColor{Light: "#FBE331", Dark: "#FBE331"}).Bold(true).Background(lipgloss.NoColor{})
	b.Title.Foreground(lipgloss.AdaptiveColor{Light: "#AF9A04", Dark: "#AF9A04"}).Bold(true).Background(lipgloss.NoColor{})

	f.Base = f.Base.Foreground(lipgloss.Color("#FBE331"))
	b.Base = f.Base.Foreground(lipgloss.Color("#AF9A04"))
	f.SelectSelector.Foreground(lipgloss.Color("#FBE331"))
	f.SelectedOption.Foreground(lipgloss.Color("#FBE331"))
	f.MultiSelectSelector.Foreground(lipgloss.Color("#FBE331"))
	f.SelectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FBE331", Dark: "#FBE331"}).SetString("✓ ")
	f.UnselectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FBE331", Dark: "#FBE331"}).SetString("• ")
	f.FocusedButton.Background(lipgloss.Color("#FBE331"))
	f.Next = f.FocusedButton.Copy()
	f.TextInput.Prompt.Foreground(lipgloss.Color("#FBE331"))

	b.SelectSelector.Foreground(lipgloss.Color("#AF9A04"))
	b.MultiSelectSelector.Foreground(lipgloss.Color("#AF9A04"))
	b.SelectedOption.Foreground(lipgloss.Color("#AF9A04"))
	b.SelectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AF9A04", Dark: "#AF9A04"}).SetString("✓ ")
	b.UnselectedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#AF9A04", Dark: "#AF9A04"}).SetString("• ")
	b.FocusedButton.Background(lipgloss.Color("#AF9A04"))
	b.Next = f.FocusedButton.Copy()
	b.TextInput.Prompt.Foreground(lipgloss.Color("#AF9A04"))
}
