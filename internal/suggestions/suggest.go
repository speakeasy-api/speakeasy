package suggestions

import (
	"context"
	goerr "errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"

	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"go.uber.org/zap"
)

var ErrNoSuggestionFound = goerr.New("no suggestion found")

const suggestionBatchSize = 5

func StartSuggest(ctx context.Context, schemaPath string, suggestionsConfig *Config, outputHints bool) error {
	fmt.Println("Validating OpenAPI spec...")
	fmt.Println()

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	schema, err = ReformatFile(schema, DetectFileType(schemaPath))
	if err != nil {
		return fmt.Errorf("failed to reformat schema file %s: %w", schemaPath, err)
	}

	vErrs, vWarns, vInfo, err := validation.Validate(schema, schemaPath, outputHints)
	if err != nil {
		return err
	}

	printValidationSummary(vErrs, vWarns, vInfo)

	toSuggestFor := vErrs
	switch suggestionsConfig.Level {
	case errors.SeverityWarn:
		toSuggestFor = append(toSuggestFor, vWarns...)
	case errors.SeverityHint:
		toSuggestFor = append(append(toSuggestFor, vWarns...), vInfo...)
	}

	// Limit the number of errors to MaxSuggestions
	if suggestionsConfig.MaxSuggestions != nil && *suggestionsConfig.MaxSuggestions < len(toSuggestFor) {
		toSuggestFor = toSuggestFor[:*suggestionsConfig.MaxSuggestions]
	}

	if len(toSuggestFor) > 0 {
		err = Suggest(schema, schemaPath, toSuggestFor, *suggestionsConfig)
		if err != nil {
			fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("cannot fetch llm suggestions: %s", err.Error())))
			return err
		}

		if suggestionsConfig.OutputFile != "" && suggestionsConfig.AutoContinue {
			fmt.Println(promptui.Styler(promptui.FGWhite)("Suggestions applied and written to " + suggestionsConfig.OutputFile))
			fmt.Println()
		}
	} else {
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Congrats! 🎊 Your spec had no issues we could detect."))
	}

	return nil
}

func Suggest(schema []byte, schemaPath string, errs []error, config Config) error {
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

	suggestionToken, fileType, err := Upload(schema, schemaPath, config.Model)
	if err != nil {
		return err
	} else {
		// Cleanup Memory Usage in LLM
		defer func() {
			Clear(suggestionToken)
		}()

		// Handle Signal Exit
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			Clear(suggestionToken)
			os.Exit(0)
		}()
	}

	suggest, err := New(suggestionToken, schemaPath, fileType, schema, config)
	if err != nil {
		return err
	}

	/**
	 * Parallelized suggestions
	 */
	if config.Parallelize {
		fmt.Println("Getting suggestions...")
		fmt.Println()

		suggest.Verbose = false

		// Request suggestions in parallel, in batches of suggestionBatchSize
		suggestions := make([]*Suggestion, len(errs))
		for i := 0; i < len(errs); i += suggestionBatchSize {
			end := int(math.Min(float64(i+suggestionBatchSize), float64(len(errs))))
			res, err := suggest.FindSuggestions(errs[i:end])
			if err != nil {
				return err
			}

			suggestions = append(suggestions, res...)
		}

		for i, err := range errs {
			suggestion := suggestions[i]

			printVErr(l, err)
			fmt.Println() // Spacing
			suggestion.Print()

			if suggestion != nil {
				fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("✓ Suggestion is valid and resolves the error"))
				fmt.Println() // Spacing

				if suggest.AwaitShouldApply() {
					newFile, err := suggest.ApplySuggestion(*suggestion)
					if err != nil {
						return err
					}

					err = suggest.CommitSuggestion(newFile)
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	}

	/**
	 * Non-parallelized suggestions
	 */
	for _, validationErr := range errs {
		if suggest.ShouldSkip(validationErr) {
			continue
		}

		printVErr(l, validationErr)

		_, newFile, err := suggest.GetSuggestionAndRevalidate(validationErr, nil)
		if err != nil {
			if goerr.Is(err, ErrNoSuggestionFound) {
				fmt.Println("Did not find a suggestion for error.")
				suggest.Skip(validationErr)
				continue
			} else {
				return err
			}
		}

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

	return nil
}

func printVErr(l *log.Logger, sourceErr error) {
	vErr := errors.GetValidationErr(sourceErr)

	if vErr != nil {
		if vErr.Severity == errors.SeverityError {
			l.Error("", zap.Error(sourceErr))
		} else if vErr.Severity == errors.SeverityWarn {
			l.Warn("", zap.Error(sourceErr))
		} else if vErr.Severity == errors.SeverityHint {
			l.Info("", zap.Error(sourceErr))
		}
	}
}

func printValidationSummary(errs []error, warns []error, info []error) {
	pluralize := func(s string, n int) string {
		if n == 1 {
			return s
		} else {
			return s + "s"
		}
	}

	stringify := func(s string, errs []error) string {
		return fmt.Sprintf("%d %s", len(errs), pluralize(s, len(errs)))
	}

	fmt.Printf(
		"Found %s, %s, and %s.\n\n",
		promptui.Styler(promptui.FGRed, promptui.FGBold)(stringify("error", errs)),
		promptui.Styler(promptui.FGYellow, promptui.FGBold)(stringify("warning", warns)),
		promptui.Styler(promptui.FGBlue, promptui.FGBold)(stringify("hint", info)),
	)
}
