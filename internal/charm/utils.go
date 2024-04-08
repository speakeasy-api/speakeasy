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

const AutoCompleteAnnotation = "autocomplete_extensions"

var OpenAPIFileExtensions = []string{".yaml", ".yml", ".json"}

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
func SchemaFilesInCurrentDir(relativeDir string, fileExtensions []string) []string {
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
		if !file.Type().IsDir() {
			for _, ext := range fileExtensions {
				if strings.HasSuffix(file.Name(), ext) {
					fileSuggestion := filepath.Join(relativeDir, file.Name())
					// allows us to support current directory relative paths
					if relativeDir == "./" {
						fileSuggestion = relativeDir + file.Name()
					}
					validFiles = append(validFiles, fileSuggestion)
				}
			}
		}
	}

	return validFiles
}

func DirsInCurrentDir(relativeDir string) []string {
	var validDirs []string
	workingDir, err := os.Getwd()
	if err != nil {
		return validDirs
	}

	targetDir := filepath.Join(workingDir, relativeDir)
	if filepath.IsAbs(relativeDir) {
		targetDir = relativeDir
	}

	files, err := os.ReadDir(targetDir)
	if err != nil {
		return validDirs
	}

	for _, file := range files {
		if file.Type().IsDir() {
			fileSuggestion := filepath.Join(relativeDir, file.Name())
			// allows us to support current directory relative paths
			if relativeDir == "./" {
				fileSuggestion = relativeDir + file.Name()
			}
			validDirs = append(validDirs, fileSuggestion)
		}
	}

	return validDirs
}

type SuggestionCallbackConfig struct {
	FileExtensions []string
	IsDirectories  bool
}

func SuggestionCallback(cfg SuggestionCallbackConfig) func(val string) []string {
	return func(val string) []string {
		var files []string
		if info, err := os.Stat(val); err == nil && info.IsDir() {
			if len(cfg.FileExtensions) > 0 {
				files = SchemaFilesInCurrentDir(val, cfg.FileExtensions)
			} else if cfg.IsDirectories {
				files = DirsInCurrentDir(val)
			}
		}

		return files
	}
}
