package styles

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"regexp"
	"strings"
)

var (
	patternToStyle = map[string]lipgloss.Style{
		"\\*": lipgloss.NewStyle().Bold(true),
		//"_":   lipgloss.NewStyle().Italic(true), // TODO: Disabled for now because values like SPEAKEASY_API_KEY get messed up
		"`": HeavilyEmphasized,
	}
)

// InjectMarkdownStyles parses the string for markdown patterns and applies the appropriate styles
// For example, the string "*bold text*" will be rendered in bold with the asterisks stripped out
// The "parent style" will then be resumed, which is the nearest preceding style to the match.
func InjectMarkdownStyles(s string) string {
	parentStyleRegex := regexp.MustCompile(`\x1b\[.*?m`)
	parentStyle := "\x1b[0m"

	for pattern, style := range patternToStyle {
		s2 := strings.Builder{}
		lastEndingIndex := 0

		codeRx := regexp.MustCompile(fmt.Sprintf("%s([^%s]+)%s", pattern, pattern, pattern))

		for _, match := range codeRx.FindAllStringSubmatchIndex(s, -1) {
			// Find the last style present in the string before the match
			before := s[lastEndingIndex : match[2]-1] // Get the portion of the string before the match, up to the symbol
			parentStyleMatches := parentStyleRegex.FindAllString(before, -1)
			if len(parentStyleMatches) > 0 {
				parentStyle = parentStyleMatches[len(parentStyleMatches)-1]
			}

			s2.WriteString(before)
			s2.WriteString(style.Render(s[match[2]:match[3]]))
			s2.WriteString(parentStyle)
			lastEndingIndex = match[3] + 1 // +1 because of closing symbol
		}

		s = s2.String() + s[lastEndingIndex:]
	}

	return s
}
