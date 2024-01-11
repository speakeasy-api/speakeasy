package utils

import "github.com/charmbracelet/lipgloss"

var (
	DocStyle = lipgloss.NewStyle().Margin(1, 2)

	TitleStyle = lipgloss.
			NewStyle().
			Foreground(Colors.Yellow).
			Bold(true)

	FocusedStyle       = lipgloss.NewStyle().Foreground(Colors.Yellow)
	FocusedStyleDimmed = FocusedStyle.Copy().Foreground(Colors.DimYellow)

	BlurredStyle = lipgloss.NewStyle().Foreground(Colors.Grey)
	HelpStyle    = BlurredStyle.Copy()

	SuccessStyle = TitleStyle.Copy().Foreground(Colors.Green)

	CursorStyle = FocusedStyleDimmed.Copy()
	NoStyle     = lipgloss.NewStyle()

	Colors = struct {
		Yellow, DimYellow, Brown, Red, DimRed, Green, DimGreen, BrightGrey, Grey, White, DimGrey lipgloss.AdaptiveColor
	}{
		Yellow:     lipgloss.AdaptiveColor{Dark: "#FBE331", Light: "#AF9A04"},
		DimYellow:  lipgloss.AdaptiveColor{Dark: "#AF9A04", Light: "#212015"},
		Brown:      lipgloss.AdaptiveColor{Dark: "#212015", Light: "#212015"},
		White:      lipgloss.AdaptiveColor{Dark: "#F3F0E3", Light: "#16150E"},
		Red:        lipgloss.AdaptiveColor{Dark: "#B92226", Light: "#54121B"},
		DimRed:     lipgloss.AdaptiveColor{Dark: "#54121B", Light: "#B92226"},
		Green:      lipgloss.AdaptiveColor{Dark: "#63AC67", Light: "#293D2A"},
		DimGreen:   lipgloss.AdaptiveColor{Dark: "#293D2A", Light: "#63AC67"},
		BrightGrey: lipgloss.AdaptiveColor{Dark: "#9E9E9E", Light: "#292821"},
		Grey:       lipgloss.AdaptiveColor{Dark: "#605E53", Light: "#605E53"},
		DimGrey:    lipgloss.AdaptiveColor{Dark: "#292821", Light: "#9E9E9E"},
	}
)
