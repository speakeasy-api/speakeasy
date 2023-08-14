package suggestions

import (
	"context"
	goerr "errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/manifoldco/promptui"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/speakeasy/internal/auth"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/validation"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

var ErrNoSuggestionFound = goerr.New("no suggestion found")

const suggestionBatchSize = 3

type allSchemasErrorSummary map[string]*SchemaErrorSummary

type SchemaErrorSummary struct {
	Error CountAndErrors `yaml:"error"`
	Warn  CountAndErrors `yaml:"warn"`
	Hint  CountAndErrors `yaml:"hint"`
}

type CountAndErrors struct {
	Count  int
	Errors []string
}

func StartSuggest(ctx context.Context, schemaPath string, isDir bool, suggestionsConfig *Config) error {
	totalErrorSummary := allSchemasErrorSummary{}

	if isDir {
		filePaths := []string{}

		walkFn := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".json" || ext == ".yaml" || ext == ".yml" {
					filePaths = append(filePaths, path)
				}
			}
			return nil
		}

		err := filepath.Walk(schemaPath, walkFn)
		if err != nil {
			return err
		}

		if suggestionsConfig.NumSpecs != nil && *suggestionsConfig.NumSpecs < len(filePaths) {
			filePaths = filePaths[:*suggestionsConfig.NumSpecs]
		}

		for _, filePath := range filePaths {
			errorSummary, err := startSuggestSchemaFile(ctx, filePath, suggestionsConfig, true)
			if err != nil {
				return err
			}

			totalErrorSummary[filePath] = errorSummary
		}
	} else {
		errorSummary, err := startSuggestSchemaFile(ctx, schemaPath, suggestionsConfig, true)
		if err != nil {
			return err
		}

		totalErrorSummary[schemaPath] = errorSummary
	}

	if suggestionsConfig.Summary {
		err := printErrorSummary(totalErrorSummary)
		if err != nil {
			return err
		}
	}

	return nil
}

func startSuggestSchemaFile(ctx context.Context, schemaPath string, suggestionsConfig *Config, outputHints bool) (*SchemaErrorSummary, error) {
	fmt.Println("Validating OpenAPI spec...")
	fmt.Println()

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	schema, err = ReformatFile(schema, DetectFileType(schemaPath))
	if err != nil {
		return nil, fmt.Errorf("failed to reformat schema file %s: %w", schemaPath, err)
	}

	vErrs, vWarns, vInfo, err := validation.Validate(schema, schemaPath, outputHints)
	if err != nil {
		return nil, err
	}

	printValidationSummary(vErrs, vWarns, vInfo)

	toSuggestFor := vErrs
	switch suggestionsConfig.Level {
	case errors.SeverityWarn:
		toSuggestFor = append(toSuggestFor, vWarns...)
	case errors.SeverityHint:
		toSuggestFor = append(append(toSuggestFor, vWarns...), vInfo...)
	}

	var errorSummary *SchemaErrorSummary
	if len(toSuggestFor) > 0 {
		errorSummary, err = Suggest(schema, schemaPath, toSuggestFor, *suggestionsConfig)
		if err != nil {
			fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("cannot fetch llm suggestions: %s", err.Error())))
			return nil, err
		}

		if suggestionsConfig.OutputFile != "" && suggestionsConfig.AutoContinue {
			fmt.Println(promptui.Styler(promptui.FGWhite)("Suggestions applied and written to " + suggestionsConfig.OutputFile))
			fmt.Println()
		}
	} else {
		fmt.Println(promptui.Styler(promptui.FGGreen, promptui.FGBold)("Congrats! 🎊 Your spec had no issues we could detect."))
	}

	return errorSummary, nil
}

func Suggest(schema []byte, schemaPath string, errs []error, config Config) (*SchemaErrorSummary, error) {
	suggestionToken := ""
	fileType := ""

	l := log.NewLogger(schemaPath)

	// local authentication should just be set in env variable
	if os.Getenv("SPEAKEASY_SERVER_URL") != "http://localhost:35290" {
		if err := auth.Authenticate(false); err != nil {
			return nil, err
		}
	}

	if _, err := GetOpenAIKey(); err != nil {
		return nil, err
	}

	suggestionToken, fileType, err := Upload(schema, schemaPath, config.Model)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	/**
	 * Parallelized suggestions
	 */
	if config.Parallelize {
		fmt.Println("Getting suggestions...")
		fmt.Println()

		suggest.Verbose = false
		continueSuggest := true
		var err error

		// Request suggestions in parallel, in batches of suggestionBatchSize
		for continueSuggest {
			continueSuggest, err = suggest.findAndApplySuggestions(l, errs[:suggestionBatchSize])
			if err != nil {
				return nil, err
			}

			errs, err = suggest.revalidate()
			if err != nil {
				return nil, err
			}
		}

		errorSummary := getSchemaErrorSummary(errs)

		return errorSummary, nil
	}

	/**
	 * Non-parallelized suggestions
	 */
	for _, validationErr := range errs {
		if !checkSuggestionCount(len(errs), suggest.suggestionCount, config.MaxSuggestions) {
			break
		}

		if suggest.shouldSkip(validationErr) {
			continue
		}

		printVErr(l, validationErr)

		_, newFile, err := suggest.getSuggestionAndRevalidate(validationErr, nil)

		if err != nil {
			if goerr.Is(err, ErrNoSuggestionFound) {
				fmt.Println("Did not find a suggestion for error.")
				suggest.skip(validationErr)
				continue
			} else {
				return nil, err
			}
		}

		if suggest.awaitShouldApply() {
			err := suggest.commitSuggestion(newFile)
			if err != nil {
				return nil, err
			}
		} else {
			suggest.skip(validationErr)
		}

		suggest.suggestionCount++

		errs, err = suggest.revalidate()
		if err != nil {
			return nil, err
		}
	}

	errorSummary := getSchemaErrorSummary(errs)

	return errorSummary, nil
}

func getSchemaErrorSummary(errs []error) *SchemaErrorSummary {
	errorSummary := &SchemaErrorSummary{}
	for _, err := range errs {
		vErr := errors.GetValidationErr(err)
		if vErr != nil {
			if vErr.Severity == errors.SeverityError {
				errorSummary.Error.Errors = append(errorSummary.Error.Errors, vErr.Error())
				errorSummary.Error.Count++
			} else if vErr.Severity == errors.SeverityWarn {
				errorSummary.Warn.Errors = append(errorSummary.Warn.Errors, vErr.Error())
				errorSummary.Warn.Count++
			} else if vErr.Severity == errors.SeverityHint {
				errorSummary.Hint.Errors = append(errorSummary.Hint.Errors, vErr.Error())
				errorSummary.Hint.Count++
			}
		}
	}

	return errorSummary
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

func checkSuggestionCount(errCount, suggestionCount int, maxSuggestions *int) bool {
	// suggestionCount < errCount meant to prevent infinite loop where applying a suggestion causes a new error
	return maxSuggestions == nil || maxSuggestions != nil && suggestionCount < *maxSuggestions && suggestionCount < errCount
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
