package log

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"regexp"
)

var (
	patternToStyle = map[string]lipgloss.Style{
		"\\*": lipgloss.NewStyle().Bold(true),
		"_":   lipgloss.NewStyle().Italic(true),
		"`":   styles.HeavilyEmphasized,
	}
)

// InjectMarkdownStyles parses the string for markdown patterns and applies the appropriate styles
// For example, the string "*bold text*" will be rendered in bold with the asterisks stripped out
// The "parent style" will then be resumed. Note that it is assumed there is only one "parent style."
// Whatever the first style is will be used to resume the original style.
func InjectMarkdownStyles(s string) string {
	// Extract the first style from the string, if present
	parentStyleRegex := regexp.MustCompile(`\x1b\[.*?m`)
	parentStyle := parentStyleRegex.FindString(s)

	for pattern, style := range patternToStyle {
		codeRx := regexp.MustCompile(fmt.Sprintf("%s([^%s]+)%s", pattern, pattern, pattern))
		s = codeRx.ReplaceAllString(s, style.Render("$1")+parentStyle) // Resume parent style after
	}

	return s
}
