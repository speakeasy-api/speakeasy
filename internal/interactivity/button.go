package interactivity

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm"
)

var (
	baseStyle    = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	blurredStyle = baseStyle.Copy().Foreground(charm.Colors.DimGrey).Background(charm.Colors.Grey)
	validStyle   = baseStyle.Copy().Foreground(charm.Colors.DimGreen).Background(charm.Colors.Green)
	invalidStyle = baseStyle.Copy().Foreground(charm.Colors.DimRed).Background(charm.Colors.Red)

	helperTextStyle = charm.Help.Copy().MarginLeft(1)
)

type Button struct {
	Label    string
	Disabled bool
	Hovered  bool
}

type ButtonWithHelperText struct {
	Button
	HelperText      string
	ShowOnlyOnHover bool
}

func (b Button) View() string {
	validnessIndicator := " ✔"
	if b.Disabled {
		validnessIndicator = " ✘"
	}

	style := blurredStyle
	if b.Hovered {
		style = validStyle

		if b.Disabled {
			style = invalidStyle
		}
	}

	return style.Render(b.Label + validnessIndicator)
}

func (b ButtonWithHelperText) View() string {
	helperText := ""
	style := helperTextStyle

	if b.Hovered || !b.ShowOnlyOnHover {
		helperText = b.HelperText

		if b.Disabled {
			style.Foreground(charm.Colors.Red)
		} else {
			style.Foreground(charm.Colors.Green)
		}
	}

	return b.Button.View() + "\n" + style.Render(helperText)
}
