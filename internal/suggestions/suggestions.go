package suggestions

import (
	"bufio"
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/iancoleman/orderedmap"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	errorTypesToSkip = []string{"validate-json-schema"}
)

type Suggestion struct {
	SuggestedFix string `json:"suggested_fix"`
	JSONPatch    string `json:"json_patch"`
	Reasoning    string `json:"reasoning"`
}

type Suggestions struct {
	Token    string
	FilePath string
	FileType string
	File     []byte
	Verbose  bool
	Config   Config

	toSkip []error
	lines  map[int]string
}

type Config struct {
	AutoContinue   bool
	MaxSuggestions *int
	Model          string
	OutputFile     string
	Parallelize    bool            // Causes all suggestions to be requested in parallel
	Level          errors.Severity // "error" will only return suggestions for errors, "warning" will return suggestions for warnings and errors, etc.
}

func New(token, filePath, fileType string, file []byte, config Config) (*Suggestions, error) {
	lineSplit := strings.Split(string(file), "\n")
	lines := make(map[int]string)
	for i, line := range lineSplit {
		lines[i+1] = line
	}

	return &Suggestions{
		Token:    token,
		FilePath: filePath,
		FileType: fileType,
		File:     file,
		Config:   config,
		Verbose:  true,
		lines:    lines,
	}, nil
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

type ErrorAndSuggestion struct {
	errorNum   int
	suggestion *Suggestion
}

// FindSuggestions returns one suggestion per given error, in order
func (s *Suggestions) FindSuggestions(errs []error) ([]*Suggestion, error) {
	suggestions := make([]*Suggestion, len(errs))
	ch := make(chan ErrorAndSuggestion)
	var wg sync.WaitGroup

	for i, err := range errs {
		wg.Add(1)
		go s.FindSuggestionAsync(err, i, ch, &wg)
	}

	// close the channel in the background
	go func() {
		wg.Wait()
		close(ch)
	}()
	// read from channel as they come in until its closed
	for res := range ch {
		suggestions[res.errorNum] = res.suggestion
	}

	return suggestions, nil
}

func (s *Suggestions) FindSuggestionAsync(err error, errorNum int, ch chan<- ErrorAndSuggestion, wg *sync.WaitGroup) {
	defer wg.Done()
	suggestion, _, err := s.GetSuggestionAndRevalidate(err, nil)
	if err != nil {
		log.Println("Suggestion request failed:", err.Error())
		return
	}

	ch <- ErrorAndSuggestion{
		errorNum:   errorNum,
		suggestion: suggestion,
	}
}

func (s *Suggestions) FindSuggestion(validationErr error, previousSuggestionContext *string) (*Suggestion, error) {
	errString := validationErr.Error()
	vErr := errors.GetValidationErr(validationErr)
	lineNumber, lineNumberErr := getLineNumber(errString)
	if lineNumberErr == nil {
		s.Log(fmt.Sprintf("\n%s\n", promptui.Styler(promptui.FGBold)("Asking for a Suggestion!")))

		return GetSuggestion(s.Token, errString, vErr.Severity, lineNumber, s.FileType, previousSuggestionContext)
	}

	return nil, nil
}

// GetSuggestionAndRevalidate returns the updated file, a list of the new validation errors if the suggestion were to be applied
func (s *Suggestions) GetSuggestionAndRevalidate(validationErr error, previousSuggestionContext *string) (*Suggestion, []byte, error) {
	suggestion, err := s.FindSuggestion(validationErr, previousSuggestionContext)
	if err != nil {
		return nil, nil, err
	}

	if s.Verbose {
		suggestion.Print()
	}

	if suggestion != nil {
		newFile, err := s.ApplySuggestion(*suggestion)
		if err != nil {
			return s.retryOnceWithMessage(validationErr, fmt.Sprintf("suggestion: %s\nerror: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
		}

		vErrs, vWarns, vInfo, err := validation.Validate(newFile, s.FilePath, true)

		if err != nil {
			return s.retryOnceWithMessage(validationErr, fmt.Sprintf("suggestion: %s\nerror: Caused validation to fail with error: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
		}

		newErrs := append(append(vErrs, vWarns...), vInfo...)
		for _, newErr := range newErrs {
			if newErr.Error() == validationErr.Error() {
				s.Log("Suggestion did not fix error.")
				return s.retryOnceWithMessage(validationErr, fmt.Sprintf("suggestion: %s\nerror: Did not resolve the original error", suggestion.JSONPatch), previousSuggestionContext)
			}
		}

		return suggestion, newFile, nil
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}

func (s *Suggestions) retryOnceWithMessage(validationErr error, msg string, previousSuggestion *string) (*Suggestion, []byte, error) {
	// Retry, but only once
	if previousSuggestion == nil {
		s.Log("Retrying...")
		return s.GetSuggestionAndRevalidate(validationErr, &msg)
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}

func (s *Suggestions) ApplySuggestion(suggestion Suggestion) ([]byte, error) {
	s.Log("Testing suggestion...")

	patch, err := jsonpatch.DecodePatch([]byte(suggestion.JSONPatch))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	original := orderedmap.New()
	if err = json.Unmarshal(s.File, &original); err != nil {
		return nil, fmt.Errorf("error unmarshaling file: %w", err)
	}

	updated, err := patch.ApplyIndent(s.File, "  ")
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling file: %w", err)
	}

	newOrder := orderedmap.New()
	if err = json.Unmarshal(updated, &newOrder); err != nil {
		return nil, fmt.Errorf("error unmarshaling file: %w", err)
	}

	// Reorder the keys in the map to match the original file
	MatchOrder(newOrder, original)

	updated, _ = json.MarshalIndent(newOrder, "", "  ")

	s.Log("Suggestion is valid!")

	return updated, nil
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

func (s *Suggestions) GetFile() ([]byte, error) {
	// Convert back to yaml from json if source file was yaml
	if s.FileType == "yaml" {
		data := orderedmap.New()
		err := json.Unmarshal(s.File, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		yamlMapSlice := jsonToYaml(*data)

		yamlFile, err := yaml.Marshal(&yamlMapSlice)
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

func (s *Suggestions) Log(message string) {
	if s.Verbose {
		fmt.Println(message)
	}
}

func (s *Suggestion) Print() {
	if s != nil && !strings.Contains(s.JSONPatch, "I cannot provide an answer") {
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggestion:"))
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGItalic)(s.SuggestedFix))
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGBold)("Explanation:"))
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGItalic)(s.Reasoning))
		fmt.Println() // extra line for spacing
		return
	}

	fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
	fmt.Println() // extra line for spacing
}
