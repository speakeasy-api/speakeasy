package log

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"golang.org/x/exp/maps"
	"regexp"
)

var (
	patternToStyle = map[string]lipgloss.Style{
		"\\*": lipgloss.NewStyle().Bold(true),
		"_":   lipgloss.NewStyle().Italic(true),
		"`":   styles.HeavilyEmphasized,
	}
)

func StyleMarkdown(s string) string {
	// Extract the first style from the string, if present
	parentStyleRegex := regexp.MustCompile(`\x1b\[.*?m`)
	parentStyle := parentStyleRegex.FindString(s)

	stopChars := "\\n"
	for _, pattern := range maps.Keys(patternToStyle) {
		stopChars += pattern
	}

	for pattern, style := range patternToStyle {
		codeRx := regexp.MustCompile(fmt.Sprintf("%s([^%s]+)%s", pattern, pattern, pattern))
		s = codeRx.ReplaceAllString(s, style.Render("$1")+parentStyle) // Resume parent style after
	}

	return s
}
