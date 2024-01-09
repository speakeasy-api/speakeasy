package quickstart

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Temporary file for themes before merging with interactive mode work. This wouldn't live under quickstart
var theme *huh.Theme

func init() {
	theme = huh.ThemeBase()
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

	f.Title.Foreground(lipgloss.Color("#FBE331")).Bold(true)
	f.Title.Background(lipgloss.Color("#FBE331")).Bold(true)
	f.Description.Foreground(lipgloss.Color("#FBE331")).Bold(true)
	f.Description.Background(lipgloss.Color("#FBE331")).Bold(true)
	f.Base.Foreground(lipgloss.Color("#FBE331"))
	// f.Option.Background(lipgloss.Color("#FBE331"))
	// f.Option.Foreground(lipgloss.Color("#FBE331"))
	// f.SelectSelector.Foreground(lipgloss.Color("#FBE331"))
	f.SelectSelector = lipgloss.NewStyle().
		SetString("* ")
	f.FocusedButton.Background(lipgloss.Color("#63AC67"))
	f.TextInput.Prompt.Foreground(lipgloss.Color("#FBE331"))
	f.TextInput.Cursor.Foreground(lipgloss.Color("#FBE331"))
}
