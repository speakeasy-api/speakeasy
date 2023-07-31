package validation

import (
	"context"
	goerr "errors"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

var ErrNoSuggestionFound = goerr.New("No suggestion found")

func ValidateOpenAPI(ctx context.Context, schemaPath string, suggestionsConfig *suggestions.Config, outputHints bool) error {
	fmt.Println("Validating OpenAPI spec...")
	fmt.Println()

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	// If we are getting suggestions, we need to file to be reformatted into json first so that the line numbers are consistent
	findSuggestions := suggestionsConfig != nil
	if findSuggestions {
		schema, err = suggestions.ReformatFile(schema, suggestions.DetectFileType(schemaPath))
		if err != nil {
			return fmt.Errorf("failed to reformat schema file %s: %w", schemaPath, err)
		}
	}

	l := log.NewLogger(schemaPath)

	hasWarnings := false

	vErrs, vWarns, vInfo, err := Validate(schema, schemaPath, outputHints)
	if err != nil {
		return err
	}

	vOutput := append(append(vErrs, vWarns...), vInfo...)

	if findSuggestions {
		err := Suggest(schema, schemaPath, vOutput, *suggestionsConfig)
		if err != nil {
			fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("cannot fetch llm suggestions: %s", err.Error())))
			return err
		}
	} else {
		// Suggest prints the errors, so we need to print them here if we're not suggesting
		for _, hint := range vInfo {
			l.Info("", zap.Error(hint))
		}
		for _, warn := range vWarns {
			hasWarnings = true
			l.Warn("", zap.Error(warn))
		}
		for _, err := range vErrs {
			l.Error("", zap.Error(err))
		}
	}

	if len(vErrs) > 0 {
		status := "OpenAPI spec invalid ✖"
		github.GenerateSummary(status, vErrs)
		return fmt.Errorf(status)
	} else {
		if findSuggestions {
			fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("No errors found in OpenAPI spec, skipping suggestions!"))
		}
	}

	if hasWarnings {
		for _, warn := range vWarns {
			if vErrs == nil {
				vErrs = []error{}
			}
			vErrs = append(vErrs, warn)
		}

		github.GenerateSummary("OpenAPI spec valid with warnings ⚠", vErrs)
		fmt.Printf("OpenAPI spec %s\n", utils.Yellow("valid with warnings ⚠"))
		return nil
	}

	github.GenerateSummary("OpenAPI spec valid ✓", nil)
	fmt.Printf("OpenAPI spec %s\n", utils.Green("valid ✓"))

	return nil
}

// Validate returns (validation errors, validation warnings, validation info, error)
func Validate(schema []byte, schemaPath string, outputHints bool) ([]error, []error, []error, error) {
	l := log.NewLogger(schemaPath)

	g, err := generate.New(generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error { return nil }, func(filename string) ([]byte, error) { return nil, nil }), generate.WithLogger(l))
	if err != nil {
		return nil, nil, nil, err
	}

	errs := g.Validate(context.Background(), schema, schemaPath, outputHints)
	var vErrs []error
	var vWarns []error
	var vInfo []error

	for _, err := range errs {
		vErr := errors.GetValidationErr(err)
		if vErr != nil {
			if vErr.Severity == errors.SeverityError {
				vErrs = append(vErrs, vErr)
			} else if vErr.Severity == errors.SeverityWarn {
				vWarns = append(vWarns, vErr)
			} else if vErr.Severity == errors.SeverityHint {
				vInfo = append(vInfo, vErr)
			}
		}

		uErr := errors.GetUnsupportedErr(err)
		if uErr != nil {
			vWarns = append(vWarns, uErr)
		}
	}

	vWarns = append(vWarns, g.GetWarnings()...)

	return vErrs, vWarns, vInfo, nil
}

func Suggest(schema []byte, schemaPath string, errs []error, config suggestions.Config) error {
	suggestionToken := ""
	fileType := ""
	totalSuggestions := 0

	l := log.NewLogger(schemaPath)

	// local authentication should just be set in env variable
	if os.Getenv("SPEAKEASY_SERVER_URL") != "http://localhost:35290" {
		if err := auth.Authenticate(false); err != nil {
			return err
		}
	}

	if _, err := suggestions.GetOpenAIKey(); err != nil {
		return err
	}

	suggestionToken, fileType, err := suggestions.Upload(schema, schemaPath)
	if err != nil {
		return err
	} else {
		// Cleanup Memory Usage in LLM
		defer func() {
			suggestions.Clear(suggestionToken)
		}()

		// Handle Signal Exit
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			suggestions.Clear(suggestionToken)
			os.Exit(0)
		}()
	}

	suggest, err := suggestions.New(suggestionToken, schemaPath, fileType, config.Model, schema, config)
	if err != nil {
		return err
	}

	for len(errs) > 0 {
		validationErr := errs[0]
		if suggest.ShouldSkip(validationErr) {
			errs = errs[1:]
			continue
		}

		vErr := errors.GetValidationErr(validationErr)
		if vErr != nil {
			if vErr.Severity == errors.SeverityError {
				l.Error("", zap.Error(validationErr))
			} else if vErr.Severity == errors.SeverityWarn {
				l.Warn("", zap.Error(validationErr))
			} else if vErr.Severity == errors.SeverityHint {
				l.Info("", zap.Error(validationErr))
			}
		}

		if config.MaxSuggestions == nil || totalSuggestions <= *config.MaxSuggestions {
			newFile, newErrors, err := GetSuggestionAndRevalidate(suggest, validationErr, nil)

			if err != nil {
				if goerr.Is(err, ErrNoSuggestionFound) {
					println("Did not find a suggestion for error.")
					suggest.Skip(validationErr)
					errs = errs[1:]
					continue
				} else {
					return err
				}
			}

			errs = newErrors

			if suggest.AwaitShouldApply() {
				err := suggest.CommitSuggestion(newFile)
				if err != nil {
					return err
				}
			} else {
				suggest.Skip(validationErr)
			}

			totalSuggestions++
		}
	}

	return nil
}

// GetSuggestionAndRevalidate returns the updated file, a list of the new validation errors if the suggestion were to be applied
func GetSuggestionAndRevalidate(s *suggestions.Suggestions, validationErr error, previousSuggestionContext *string) ([]byte, []error, error) {
	suggestion, err := s.FindSuggestion(validationErr, previousSuggestionContext)
	if err != nil {
		return nil, nil, err
	}

	suggestions.Print(suggestion, validationErr)

	if suggestion != nil {
		newFile, err := s.ApplySuggestion(*suggestion)
		if err != nil {
			if previousSuggestionContext == nil {
				return retryOnceWithMessage(s, validationErr, fmt.Sprintf("suggestion: %s\nerror: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
			}
			return nil, nil, ErrNoSuggestionFound
		}

		vErrs, vWarns, vInfo, err := Validate(newFile, s.FilePath, true)

		if err != nil {
			if previousSuggestionContext == nil {
				return retryOnceWithMessage(s, validationErr, fmt.Sprintf("suggestion: %s\nerror: Caused validation to fail with error: %s", suggestion.JSONPatch, err.Error()), previousSuggestionContext)
			}
			return nil, nil, ErrNoSuggestionFound
		}

		newErrs := append(append(vErrs, vWarns...), vInfo...)
		for _, newErr := range newErrs {
			if newErr.Error() == validationErr.Error() {
				fmt.Println("Suggestion did not fix error.")
				if previousSuggestionContext == nil {
					return retryOnceWithMessage(s, validationErr, fmt.Sprintf("suggestion: %s\nerror: Did not resolve the original error", suggestion.JSONPatch), previousSuggestionContext)
				}
				return nil, nil, ErrNoSuggestionFound
			}
		}

		return newFile, newErrs, nil
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}

func retryOnceWithMessage(s *suggestions.Suggestions, validationErr error, msg string, previousSuggestion *string) ([]byte, []error, error) {
	// Retry, but only once
	if previousSuggestion == nil {
		println("Retrying...")
		return GetSuggestionAndRevalidate(s, validationErr, &msg)
	} else {
		return nil, nil, ErrNoSuggestionFound
	}
}
