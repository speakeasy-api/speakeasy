package overlay

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Block indentation is stripped on decode, so leading whitespace means more-indented.
func hasMoreIndentedLine(value string) bool {
	for _, line := range strings.Split(value, "\n") {
		if line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			return true
		}
	}

	return false
}

// yaml.v3 injects a line break before more-indented lines in folded scalars on every encode; literal blocks round trip unchanged.
func stabilizeFoldedScalars(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.ScalarNode && node.Style == yaml.FoldedStyle && hasMoreIndentedLine(node.Value) {
		node.Style = yaml.LiteralStyle
	}

	for _, child := range node.Content {
		stabilizeFoldedScalars(child)
	}
}
