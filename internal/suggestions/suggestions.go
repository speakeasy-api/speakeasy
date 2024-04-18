package suggestions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	golog "log"
	"os"
	"strconv"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/iancoleman/orderedmap"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"
)

var errorTypesToSkip = []string{"validate-json-schema"}

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

	isRemote            bool
	suggestionCount     int
	validationLoopCount int
	toSkip              []error
	lines               map[int]string
}

type Config struct {
	AutoContinue    bool
	MaxSuggestions  *int
	Model           string
	OutputFile      string
	Parallelize     bool            // Causes all suggestions (up to MaxSuggestions) to be requested in parallel
	Level           errors.Severity // "error" will only return suggestions for errors, "warning" will return suggestions for warnings and errors, etc.
	ValidationLoops *int
	NumSpecs        *int
	Summary         bool
}

type errorAndCommentLineNumber struct {
	error      error
	lineNumber int
}

func New(token, filePath, fileType string, file []byte, isRemote bool, config Config) (*Suggestions, error) {
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
		isRemote: isRemote,
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

func (s *Suggestions) awaitShouldApply() bool {
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

func (s *Suggestions) commitSuggestion(newFile []byte) error {
	s.File = newFile

	file, err := s.getFile()
	if err != nil {
		return err
	}

	// Write modified file to the path specified in config.OutputFile, if provided
	if s.Config.OutputFile != "" {
		err = os.WriteFile(s.Config.OutputFile, file, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}
	}

	return nil
}

type ErrorAndSuggestion struct {
	errorNum   int
	suggestion *Suggestion
}

func (s *Suggestions) findAndApplySuggestions(ctx context.Context, l *log.Logger, errsWithLineNums []errorAndCommentLineNumber) (bool, error) {
	errs := make([]error, len(errsWithLineNums))
	for i, errWithLineNum := range errsWithLineNums {
		errs[i] = errWithLineNum.error
	}

	res, continueSuggest, err := s.findSuggestions(ctx, errs)
	if err != nil {
		return false, err
	}

	for i, err := range errsWithLineNums {
		suggestion := res[i]

		printVErr(l, err)
		fmt.Println() // Spacing
		suggestionFound := suggestion.print()
		if suggestionFound {
			fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("✓ Suggestion is valid and resolves the error"))
			fmt.Println() // Spacing
		}

		if suggestion != nil && s.awaitShouldApply() {
			newFile, err := s.applySuggestion(*suggestion)
			if err != nil {
				return false, err
			}

			err = s.commitSuggestion(newFile)
			if err != nil {
				return false, err
			}
		}
	}

	s.validationLoopCount++

	if s.Config.ValidationLoops != nil && s.validationLoopCount >= *s.Config.ValidationLoops {
		return false, nil
	}

	return continueSuggest, nil
}

// FindSuggestions returns one suggestion per given error, in order
func (s *Suggestions) findSuggestions(ctx context.Context, errs []error) ([]*Suggestion, bool, error) {
	suggestions := make([]*Suggestion, len(errs))
	ch := make(chan ErrorAndSuggestion)
	var wg sync.WaitGroup

	for i, err := range errs {
		wg.Add(1)
		go s.findSuggestionAsync(ctx, err, i, ch, &wg)
	}

	// close the channel in the background
	go func() {
		wg.Wait()
		close(ch)
	}()
	// read from channel as they come in until its closed
	continueSuggest := true
	for res := range ch {
		if !checkSuggestionCount(len(errs), s.suggestionCount, s.Config.MaxSuggestions) {
			continueSuggest = false
			break
		}
		suggestions[res.errorNum] = res.suggestion
		s.suggestionCount++
	}

	return suggestions, continueSuggest, nil
}

func (s *Suggestions) findSuggestionAsync(ctx context.Context, err error, errorNum int, ch chan<- ErrorAndSuggestion, wg *sync.WaitGroup) {
	defer wg.Done()
	suggestion, _, err := s.getSuggestionAndRevalidate(ctx, err, nil)
	if err != nil {
		golog.Println("Suggestion request failed:", err.Error())
		return
	}

	ch <- ErrorAndSuggestion{
		errorNum:   errorNum,
		suggestion: suggestion,
	}
}

func (s *Suggestions) findSuggestion(validationErr error, previousSuggestionContext *string) (*Suggestion, error) {
	errString := validationErr.Error()
	vErr := errors.GetValidationErr(validationErr)
	lineNumber, lineNumberErr := getLineNumber(errString)
	if lineNumberErr == nil {
		s.log(fmt.Sprintf("\n%s\n", promptui.Styler(promptui.FGBold)("Asking for a Suggestion!")))

		return GetSuggestion(s.Token, errString, vErr.Severity, lineNumber, s.FileType, previousSuggestionContext)
	}

	return nil, nil
}

// GetSuggestionAndRevalidate returns the Suggestion and updated file
func (s *Suggestions) getSuggestionAndRevalidate(ctx context.Context, validationErr error, previousSuggestionContext *string) (*Suggestion, []byte, error) {
	suggestion, err := s.findSuggestion(validationErr, previousSuggestionContext)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrNoSuggestionFound, err)
	}

	if s.Verbose {
		suggestion.print()
	}

	if suggestion != nil {
		newFile, err := s.applySuggestion(*suggestion)
		if err != nil {
			return s.retryOnceWithMessage(ctx, validationErr, fmt.Sprintf("suggestion: %s\nerror: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
		}

		newErrs, err := validate(ctx, newFile, s.FilePath, s.Config.Level, s.isRemote, false)
		if err != nil {
			return s.retryOnceWithMessage(ctx, validationErr, fmt.Sprintf("suggestion: %s\nerror: Caused validation to fail with error: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
		}

		for _, newErr := range newErrs {
			if newErr.Error() == validationErr.Error() {
				s.log("Suggestion did not fix error.")
				return s.retryOnceWithMessage(ctx, validationErr, fmt.Sprintf("suggestion: %s\nerror: Did not resolve the original error", suggestion.JSONPatch), previousSuggestionContext)
			}
		}

		return suggestion, newFile, nil
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}

func (s *Suggestions) revalidate(ctx context.Context, printSummary bool) ([]errorAndCommentLineNumber, error) {
	errs, err := validate(ctx, s.File, s.FilePath, s.Config.Level, s.isRemote, printSummary)
	if err != nil {
		return nil, err
	}

	var errsWithLineNums []errorAndCommentLineNumber
	if s.FileType == "yaml" {
		yamlFile, err := convertJsonToYaml(s.File)
		if err != nil {
			return nil, err
		}

		yamlErrs, err := validate(ctx, yamlFile, s.FilePath, s.Config.Level, s.isRemote, false)
		if err != nil {
			return nil, err
		}

		errsWithLineNums = updateErrsWithLineNums(errs, yamlErrs)
	} else {
		for _, err := range errs {
			vErr := errors.GetValidationErr(err)
			if vErr != nil {
				errsWithLineNums = append(errsWithLineNums, errorAndCommentLineNumber{
					error:      err,
					lineNumber: vErr.LineNumber,
				})
			}
		}
	}

	return errsWithLineNums, nil
}

func validate(ctx context.Context, schema []byte, schemaPath string, level errors.Severity, isRemote, printSummary bool) ([]error, error) {
	res, err := validation.Validate(ctx, log.From(ctx), schema, schemaPath, nil, isRemote, "speakeasy-recommended", "")
	if err != nil {
		return nil, fmt.Errorf("failed to validate YAML: %v", err)
	}

	if printSummary {
		printValidationSummary(res.Errors, res.Warnings, res.Infos)
	}

	errs := res.Errors
	switch level {
	case errors.SeverityWarn:
		errs = append(errs, res.Warnings...)
	case errors.SeverityHint:
		errs = append(append(errs, res.Warnings...), res.Infos...)
	}

	return errs, nil
}

/*
updateErrsWithYamlLineNums returns the old errors with the line numbers of the new errors, provided the errors are
equivalent except for the line numbers.
*/
func updateErrsWithLineNums(oldErrs []error, newErrs []error) []errorAndCommentLineNumber {
	// Return each error in the JSON document along with the line number of the corresponding error in the YAML document
	var retErrs []errorAndCommentLineNumber
	for _, oldErr := range oldErrs {
		voErr := errors.GetValidationErr(oldErr)
		if voErr == nil {
			continue
		}

		for _, newErr := range newErrs {
			vnErr := errors.GetValidationErr(newErr)
			if vnErr == nil {
				continue
			}

			if validationErrsEqualExceptLineNumber(*voErr, *vnErr) {
				retErrs = append(retErrs, errorAndCommentLineNumber{
					error:      voErr,
					lineNumber: vnErr.LineNumber,
				})
			}
		}
	}

	return retErrs
}

func validationErrsEqualExceptLineNumber(err1, err2 errors.ValidationError) bool {
	return err1.Severity == err2.Severity && err1.Message == err2.Message && err1.Rule == err2.Rule && err1.Path == err2.Path
}

func (s *Suggestions) retryOnceWithMessage(ctx context.Context, validationErr error, msg string, previousSuggestion *string) (*Suggestion, []byte, error) {
	// Retry, but only once
	if previousSuggestion == nil {
		s.log("Retrying...")
		return s.getSuggestionAndRevalidate(ctx, validationErr, &msg)
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}

func printErrorSummary(totalErrorSummary allSchemasErrorSummary) error {
	yamlBytes, err := yaml.Marshal(totalErrorSummary)
	if err != nil {
		return err
	}
	fmt.Println(string(yamlBytes))
	return nil
}

func (s *Suggestions) applySuggestion(suggestion Suggestion) ([]byte, error) {
	s.log("Testing suggestion...")

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
	matchOrder(newOrder, original)

	updated, _ = json.MarshalIndent(newOrder, "", "  ")

	s.log("Suggestion is valid!")

	return updated, nil
}

func (s *Suggestions) getFile() ([]byte, error) {
	// Convert back to yaml from json if source file was yaml
	if s.FileType == "yaml" {
		yamlFile, err := convertJsonToYaml(s.File)
		if err != nil {
			return nil, err
		}
		return yamlFile, nil
	} else {
		return s.File, nil
	}
}

func (s *Suggestions) skip(err error) {
	s.toSkip = append(s.toSkip, err)
}

// ShouldSkip TODO: Make this work even when line numbers subsequently change
func (s *Suggestions) shouldSkip(err error) bool {
	for _, skipErrType := range errorTypesToSkip {
		if strings.Contains(err.Error(), skipErrType) {
			return true
		}
	}

	return slices.Contains(s.toSkip, err)
}

func (s *Suggestions) log(message string) {
	if s.Verbose {
		fmt.Println(message)
	}
}

func (s *Suggestion) print() bool {
	if s != nil && !strings.Contains(s.JSONPatch, "I cannot provide an answer") {
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Suggestion:"))
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGItalic)(s.SuggestedFix))
		fmt.Println() // extra line for spacing
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGBold)("Explanation:"))
		fmt.Println(promptui.Styler(promptui.FGYellow, promptui.FGItalic)(s.Reasoning))
		fmt.Println() // extra line for spacing
	} else {
		fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)("No Suggestion Found"))
		fmt.Println() // extra line for spacing
		return false
	}
	return true
}
