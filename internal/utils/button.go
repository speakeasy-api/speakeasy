package utils

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle    = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	blurredStyle = baseStyle.Copy().Foreground(Colors.DimGrey).Background(Colors.Grey)
	validStyle   = baseStyle.Copy().Foreground(Colors.DimGreen).Background(Colors.Green)
	invalidStyle = baseStyle.Copy().Foreground(Colors.DimRed).Background(Colors.Red)

	helperTextStyle = HelpStyle.Copy().MarginLeft(1)
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
			style.Foreground(Colors.Red)
		} else {
			style.Foreground(Colors.Green)
		}
	}

	return b.Button.View() + "\n" + style.Render(helperText)
}
