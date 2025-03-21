package styles

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/term"
)

var (
	Margins = lipgloss.NewStyle().Margin(1, 2)

	HeavilyEmphasized = lipgloss.
				NewStyle().
				Foreground(Colors.Yellow).
				Bold(true)

	Emphasized = HeavilyEmphasized.Foreground(Colors.WhiteBlackAdaptive)

	Success = lipgloss.NewStyle().Foreground(Colors.Green).Bold(true)
	Info    = lipgloss.NewStyle().Foreground(Colors.Blue)
	Warning = lipgloss.NewStyle().Foreground(Colors.Yellow)
	Error   = lipgloss.NewStyle().Foreground(Colors.Red)

	Focused       = lipgloss.NewStyle().Foreground(Colors.Yellow)
	FocusedDimmed = Focused.Foreground(Colors.DimYellow)

	Dimmed       = lipgloss.NewStyle().Foreground(Colors.Grey)
	DimmedItalic = Dimmed.Italic(true)
	Help         = DimmedItalic

	Cursor = FocusedDimmed
	None   = lipgloss.NewStyle()

	Colors = struct {
		Yellow, DimYellow, SpeakeasyPrimary, SpeakeasySecondary, Red, DimRed, Green, DimGreen, BrightGrey, Grey, WhiteBlackAdaptive, DimGrey, Blue, DimBlue lipgloss.AdaptiveColor
	}{
		Yellow:             lipgloss.AdaptiveColor{Dark: "#FBE331", Light: "#C0A802"},
		DimYellow:          lipgloss.AdaptiveColor{Dark: "#AF9A04", Light: "#AF9A04"},
		SpeakeasyPrimary:   lipgloss.AdaptiveColor{Dark: "#FBE331", Light: "#212015"},
		SpeakeasySecondary: lipgloss.AdaptiveColor{Dark: "#212015", Light: "#FBE331"},
		WhiteBlackAdaptive: lipgloss.AdaptiveColor{Dark: "#F3F0E3", Light: "#16150E"},
		Red:                lipgloss.AdaptiveColor{Dark: "#D93337", Light: "#54121B"},
		DimRed:             lipgloss.AdaptiveColor{Dark: "#54121B", Light: "#D93337"},
		Green:              lipgloss.AdaptiveColor{Dark: "#63AC67", Light: "#5B8537"},
		DimGreen:           lipgloss.AdaptiveColor{Dark: "#293D2A", Light: "#63AC67"},
		BrightGrey:         lipgloss.AdaptiveColor{Dark: "#B4B2A6", Light: "#4B4A3F"},
		Grey:               lipgloss.AdaptiveColor{Dark: "#8A887D", Light: "#68675F"},
		DimGrey:            lipgloss.AdaptiveColor{Dark: "#4B4A3F", Light: "#B4B2A6"},
		Blue:               lipgloss.AdaptiveColor{Dark: "#679FE1", Light: "#1D2A3A"},
		DimBlue:            lipgloss.AdaptiveColor{Dark: "#1D2A3A", Light: "#679FE1"},
	}
)

func TerminalWidth() int {
	termWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))
	return termWidth
}

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
	s := Success.Render(utils.CapitalizeFirst(heading))
	for _, line := range additionalLines {
		s += "\n" + Dimmed.Render(line)
	}

	return MakeBoxed(s, Colors.Green, lipgloss.Center)
}

func RenderWarningMessage(heading string, additionalLines ...string) string {
	s := Warning.Render(utils.CapitalizeFirst(heading))
	for _, line := range additionalLines {
		s += "\n" + Dimmed.Render(line)
	}

	return MakeBoxed(s, Colors.Yellow, lipgloss.Center)
}

func RenderInfoMessage(heading string, additionalLines ...string) string {
	s := lipgloss.NewStyle().Foreground(Colors.Blue).Bold(true).Render(utils.CapitalizeFirst(heading))
	for _, line := range additionalLines {
		s += "\n" + lipgloss.NewStyle().Foreground(Colors.Blue).Render(line)
	}

	return MakeBoxed(s, Colors.Blue, lipgloss.Center)
}

func RenderErrorMessage(heading string, alignment lipgloss.Position, additionalLines ...string) string {
	s := lipgloss.NewStyle().Foreground(Colors.Red).Bold(true).Render(utils.CapitalizeFirst(heading))
	for _, line := range additionalLines {
		s += "\n" + lipgloss.NewStyle().Foreground(Colors.Red).Render(line)
	}

	return MakeBoxed(s, Colors.Red, alignment)
}

func RenderInstructionalError(heading string, additionalLines ...string) string {
	s := Error.Render(utils.CapitalizeFirst(heading + "\n"))
	for _, line := range additionalLines {
		s += "\n\n" + Error.Render(line)
	}

	return MakeBoxed(s, Colors.Red, lipgloss.Left)
}

func RenderInstructionalMessage(heading string, additionalLines ...string) string {
	s := Info.Render(utils.CapitalizeFirst(heading + "\n"))
	for _, line := range additionalLines {
		s += "\n\n" + Info.Render(line)
	}

	return MakeBoxed(s, Colors.Blue, lipgloss.Left)
}

func MakeBold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

func MakeBoxed(s string, borderColor lipgloss.AdaptiveColor, alignment lipgloss.Position) string {
	// We need to do this before boxing things because the sizing can change
	s = InjectMarkdownStyles(s)

	termWidth := TerminalWidth() - 2     // Leave room for padding (if the terminal is too small to fit, we need to wrap)
	stringWidth := lipgloss.Width(s) + 2 // Account for padding (on the other hand, if the terminal is wide enough, add back in the space so it doesn't needlessly wrap)
	w := min(termWidth, stringWidth)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		AlignHorizontal(alignment).
		Width(w).
		Render(s)
}

// MakeSection returns a string enclosed in a top and bottom border with a title
func MakeSection(title, content string, color lipgloss.AdaptiveColor) string {
	titleLine := MakeBreak(title, '─', color, true)
	footerLine := MakeBreak(title, '─', color, false)

	return fmt.Sprintf("%s\n\n%s\n\n%s", titleLine, content, footerLine)
}

func MakeBreak(heading string, character rune, color lipgloss.AdaptiveColor, isStart bool) string {
	termWidth := TerminalWidth()

	line := ""
	if heading == "" {
		line = strings.Repeat(string(character), termWidth)
	} else {
		separator := " ↑ "
		if isStart {
			separator = " ↓ "
		}
		borderWidth := (termWidth - lipgloss.Width(heading) - 2*lipgloss.Width(separator)) / 2
		borderString := strings.Repeat(string(character), borderWidth)
		line = fmt.Sprintf("%s%s%s%s%s", borderString, separator, heading, separator, borderString)
	}

	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(line)
}

func RenderKeymapLegend(keys []string, descriptions []string) string {
	var s string
	for i, key := range keys {
		s += key + " " + Dimmed.Render(descriptions[i]) + "  "
	}
	return s
}

// Renders the support@speakeasy.com email address with emphasis for interactive
// output and as a normal string in GitHub Actions.
func RenderSupportEmail() string {
	if env.IsGithubAction() {
		return "support@speakeasy.com"
	}

	return Emphasized.Render("support@speakeasy.com")
}

func RenderSalesEmail() string {
	if env.IsGithubAction() {
		return "sales@speakeasy.com"
	}

	return Emphasized.Render("sales@speakeasy.com")
}
