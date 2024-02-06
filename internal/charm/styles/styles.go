package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

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

func LeftBorder(color lipgloss.TerminalColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true). // Left border only
		BorderForeground(color).
		PaddingLeft(1)
}

func SeverityToStyle(severity errors.Severity) lipgloss.Style {
	switch severity {
	case errors.SeverityError:
		return Error
	case errors.SeverityWarn:
		return Warning
	case errors.SeverityHint:
		return Info
	default:
		return Info
	}
}

func RenderSuccessMessage(heading string, additionalLines ...string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Green).
		Padding(0, 1).
		AlignHorizontal(lipgloss.Center)

	s := Success.Render(utils.CapitalizeFirst(heading))
	for _, line := range additionalLines {
		s += "\n" + Dimmed.Render(line)
	}

	return boxStyle.Render(s)
}

func KeymapLegend(keys []string, descriptions []string) string {
	var s string
	for i, key := range keys {
		s += key + " " + Dimmed.Render(descriptions[i]) + "  "
	}
	return s
}
