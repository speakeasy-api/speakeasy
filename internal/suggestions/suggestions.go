package suggestions

import (
	"bufio"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	json "github.com/json-iterator/go"
)

var (
	errorTypesToSkip = []string{"validate-json-schema"}
)

type Suggestions struct {
	Token    string
	FilePath string
	FileType string
	Model    string
	File     []byte
	Config   Config

	toSkip []error
	lines  map[int]string
}

type Config struct {
	AutoContinue   bool
	MaxSuggestions *int
	Model          string
	OutputFile     string
}

func New(token, filePath, fileType, model string, file []byte, config Config) (*Suggestions, error) {
	lineSplit := strings.Split(string(file), "\n")
	lines := make(map[int]string)
	for i, line := range lineSplit {
		lines[i+1] = line
	}

	return &Suggestions{
		Token:    token,
		FilePath: filePath,
		FileType: fileType,
		Model:    model,
		File:     file,
		Config:   config,
		lines:    lines,
	}, nil
}

func ReformatFile(file []byte, fileType string) ([]byte, error) {
	// If source file is yaml, convert to json
	if strings.Contains(fileType, "yaml") {
		yamlFile := make(map[string]interface{})
		err := yaml.Unmarshal(file, &yamlFile)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
		}

		jsonFile, err := json.Marshal(yamlFile)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %v", err)
		}

		file = jsonFile
	}

	// Ensure file format is consistent by applying a no-op patch
	patch, _ := jsonpatch.DecodePatch([]byte("[]"))
	file, _ = patch.ApplyIndent(file, "  ")

	return file, nil
}

func getLineNumber(errStr string) (int, error) {
	lineStr := strings.Split(errStr, "[line ")
	if len(lineStr) < 2 {
		return 0, fmt.Errorf("line number cannot be found in err %s", errStr)
	}

	lineNumStr := strings.Split(lineStr[1], "]")[0]
	lineNum, err := strconv.Atoi(lineNumStr)
	if err != nil {
		return 0, err
	}

	return lineNum, nil
}

func (s *Suggestions) AwaitShouldApply() bool {
	if s.Config.AutoContinue {
		return true
	}
	if s.Config.OutputFile == "" {
		fmt.Println()
		fmt.Print(promptui.Styler(promptui.FGCyan, promptui.FGBold)("🐝 Press 'Enter' to continue"))
		fmt.Println()

		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return true
	} else {
		fmt.Println()
		fmt.Print(promptui.Styler(promptui.FGCyan, promptui.FGBold)("🐝 Apply suggestion? y/n"))
		fmt.Println()

		bytes, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
		if err != nil {
			return false
		}

		return string(bytes) == "y\n"
	}
}

func (s *Suggestions) CommitSuggestion(newFile []byte) error {
	s.File = newFile

	file, err := s.GetFile()
	if err != nil {
		return err
	}

	// Write modified file to the path specified in config.OutputFile, if provided
	if s.Config.OutputFile != "" {
		err = os.WriteFile(s.Config.OutputFile, file, 0644)
		if err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}
	}

	return nil
}

func DetectFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".yaml", ".yml":
		return "text/yaml"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func (s *Suggestions) FindSuggestion(err error, previousSuggestionContext *string) (*Suggestion, error) {
	errString := err.Error()
	vErr := errors.GetValidationErr(err)
	lineNumber, lineNumberErr := getLineNumber(errString)
	if lineNumberErr == nil {
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGBold)("Asking for a Suggestion!"))
		fmt.Println() // extra line for spacing

		return GetSuggestion(s.Token, errString, vErr.Severity, lineNumber, s.FileType, s.Model, previousSuggestionContext)
	}

	return nil, nil
}

func Print(suggestion *Suggestion, suggestionErr error) {
	if suggestion != nil && !strings.Contains(suggestion.JSONPatch, "I cannot provide an answer") {
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggestion:"))
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGItalic)(suggestion.SuggestedFix))
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGBold)("Explanation:"))
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGItalic)(suggestion.Reasoning))
		fmt.Println() // extra line for spacing
		return
	} else {
		if suggestionErr != nil {
			if strings.Contains(suggestionErr.Error(), "401") || strings.Contains(suggestionErr.Error(), "403") {
				fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("No Suggestion Found: %s", suggestionErr.Error())))
				return
			}
		}
	}
	fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
	fmt.Println() // extra line for spacing
}

func (s *Suggestions) ApplySuggestion(suggestion Suggestion) ([]byte, error) {
	fmt.Println("Testing suggestion...")
	patch, err := jsonpatch.DecodePatch([]byte(suggestion.JSONPatch))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	updated, err := patch.ApplyIndent(s.File, "  ")
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	fmt.Println("Suggestion is valid!")

	return updated, nil
}

func linesToString(lines map[int]string) string {
	var sb strings.Builder
	for i := 1; i <= len(lines); i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}
	return sb.String()
}

func (s *Suggestions) GetFile() ([]byte, error) {
	// Convert back to yaml from json if source file was yaml
	if s.FileType == "yaml" {
		var data interface{}
		err := json.Unmarshal(s.File, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		yamlFile, err := yaml.Marshal(&data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal YAML: %v", err)
		}

		return yamlFile, nil
	} else {
		return s.File, nil
	}
}

func (s *Suggestions) Skip(err error) {
	s.toSkip = append(s.toSkip, err)
}

// ShouldSkip TODO: Make this work even when line numbers subsequently change
func (s *Suggestions) ShouldSkip(err error) bool {
	for _, skipErrType := range errorTypesToSkip {
		if strings.Contains(err.Error(), skipErrType) {
			return true
		}
	}

	return slices.Contains(s.toSkip, err)
}
