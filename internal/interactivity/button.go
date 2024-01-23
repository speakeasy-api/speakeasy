package interactivity

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

var (
	baseStyle    = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	blurredStyle = baseStyle.Copy().Foreground(styles.Colors.DimGrey).Background(styles.Colors.Grey)
	validStyle   = baseStyle.Copy().Foreground(styles.Colors.DimGreen).Background(styles.Colors.Green)
	invalidStyle = baseStyle.Copy().Foreground(styles.Colors.DimRed).Background(styles.Colors.Red)

	helperTextStyle = styles.Help.Copy().MarginLeft(1)
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
			style.Foreground(styles.Colors.Red)
		} else {
			style.Foreground(styles.Colors.Green)
		}
	}

	return b.Button.View() + "\n" + style.Render(helperText)
}
