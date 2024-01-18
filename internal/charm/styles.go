package charm

import "github.com/charmbracelet/lipgloss"

var (
	Margins = lipgloss.NewStyle().Margin(1, 2)

	HeavilyEmphasized = lipgloss.
				NewStyle().
				Foreground(Colors.Yellow).
				Bold(true)

	Emphasized = HeavilyEmphasized.Copy().Foreground(Colors.White)

	Info    = Emphasized.Copy().Foreground(Colors.Blue)
	Warning = Emphasized.Copy().Foreground(Colors.Yellow)
	Error   = Emphasized.Copy().Foreground(Colors.Red)

	Focused       = lipgloss.NewStyle().Foreground(Colors.Yellow)
	FocusedDimmed = Focused.Copy().Foreground(Colors.DimYellow)

	Dimmed       = lipgloss.NewStyle().Foreground(Colors.Grey)
	DimmedItalic = Dimmed.Copy().Italic(true)
	Help         = DimmedItalic.Copy()

	Success = Emphasized.Copy().Foreground(Colors.Green)

	Cursor = FocusedDimmed.Copy()
	None   = lipgloss.NewStyle()

	Colors = struct {
		Yellow, DimYellow, Brown, Red, DimRed, Green, DimGreen, BrightGrey, Grey, White, DimGrey, Blue, DimBlue lipgloss.AdaptiveColor
	}{
		Yellow:     lipgloss.AdaptiveColor{Dark: "#FBE331", Light: "#AF9A04"},
		DimYellow:  lipgloss.AdaptiveColor{Dark: "#AF9A04", Light: "#212015"},
		Brown:      lipgloss.AdaptiveColor{Dark: "#212015", Light: "#212015"},
		White:      lipgloss.AdaptiveColor{Dark: "#F3F0E3", Light: "#16150E"},
		Red:        lipgloss.AdaptiveColor{Dark: "#D93337", Light: "#54121B"},
		DimRed:     lipgloss.AdaptiveColor{Dark: "#54121B", Light: "#D93337"},
		Green:      lipgloss.AdaptiveColor{Dark: "#63AC67", Light: "#293D2A"},
		DimGreen:   lipgloss.AdaptiveColor{Dark: "#293D2A", Light: "#63AC67"},
		BrightGrey: lipgloss.AdaptiveColor{Dark: "#B4B2A6", Light: "#4B4A3F"},
		Grey:       lipgloss.AdaptiveColor{Dark: "#8A887D", Light: "#BAB8AA"},
		DimGrey:    lipgloss.AdaptiveColor{Dark: "#4B4A3F", Light: "#B4B2A6"},
		Blue:       lipgloss.AdaptiveColor{Dark: "#679FE1", Light: "#1D2A3A"},
		DimBlue:    lipgloss.AdaptiveColor{Dark: "#1D2A3A", Light: "#679FE1"},
	}
)
