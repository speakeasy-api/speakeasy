package charm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

func NewBranchPrompt(title string, output *bool) *huh.Group {
	return huh.NewGroup(huh.NewConfirm().
		Title(title).
		Affirmative("Yes.").
		Negative("No.").
		Value(output))
}

func NewSelectPrompt(title string, description string, options []huh.Option[string], output *string) *huh.Group {
	group := huh.NewGroup(huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(output))

	if description != "" {
		group = group.Description(description + "\n")
	}

	return group
}

func FormatCommandTitle(title string, description string) string {
	titleStyle := lipgloss.NewStyle().Foreground(styles.Focused.GetForeground()).Bold(true)
	descriptionStyle := lipgloss.NewStyle().Foreground(styles.Dimmed.GetForeground()).Italic(true)
	header := titleStyle.Render(title)
	header += "\n" + descriptionStyle.Render(description)
	return header
}

func NewInput() *huh.Input {
	return huh.NewInput().Prompt(" ").Inline(true)
}

func FormatEditOption(text string) string {
	return fmt.Sprintf("✎ %s", text)
}

func FormatNewOption(text string) string {
	return fmt.Sprintf("+ %s", text)
}

// Populates tab complete for schema files in the relative directory
func SchemaFilesInCurrentDir(relativeDir string) []string {
	var validFiles []string
	workingDir, err := os.Getwd()
	if err != nil {
		return validFiles
	}

	targetDir := filepath.Join(workingDir, relativeDir)

	files, err := os.ReadDir(targetDir)
	if err != nil {
		return validFiles
	}

	for _, file := range files {
		if !file.Type().IsDir() && (strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".json")) {
			validFiles = append(validFiles, filepath.Join(relativeDir, file.Name()))
		}
	}

	return validFiles
}

func SuggestionCallback(val string) []string {
	var files []string
	if info, err := os.Stat(val); err == nil && info.IsDir() {
		files = SchemaFilesInCurrentDir(val)
	}

	return files
}
