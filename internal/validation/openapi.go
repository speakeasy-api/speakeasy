package validation

import (
	"context"
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

type SuggestionsConfig struct {
	AutoContinue bool
}

func ValidateOpenAPI(ctx context.Context, schemaPath string, suggestionsConfig *SuggestionsConfig) error {
	fmt.Println("Validating OpenAPI spec...")

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	l := log.NewLogger(schemaPath)

	g, err := generate.New(generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error { return nil }, func(filename string) ([]byte, error) { return nil, nil }), generate.WithLogger(l))
	if err != nil {
		return err
	}

	hasWarnings := false
	errs := g.Validate(context.Background(), schema, schemaPath)
	if len(errs) > 0 {
		hasErrors := false
		findSuggestions := false
		if suggestionsConfig != nil {
			findSuggestions = true
		}
		suggestionToken := ""
		fileType := ""

		if findSuggestions {
			// local authentication should just be set in env variable
			if os.Getenv("SPEAKEASY_SERVER_URL") != "http://localhost:35290" {
				if err := auth.Authenticate(false); err != nil {
					return err
				}
			}

			if _, err := suggestions.GetOpenAIKey(); err != nil {
				return err
			}

			if findSuggestions {
				suggestionToken, fileType, err = suggestions.Upload(schemaPath)
				if err != nil {
					fmt.Println(promptui.Styler(promptui.FGRed, promptui.FGBold)(fmt.Sprintf("cannot fetch llm suggestions: %s", err.Error())))
					findSuggestions = false
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
			}
		}

		for _, err := range errs {
			vErr := errors.GetValidationErr(err)
			if vErr != nil {
				if vErr.Severity == errors.SeverityError {
					hasErrors = true
					l.Error("", zap.Error(err))
					if findSuggestions {
						suggestions.FindSuggestion(err, suggestionToken, fileType, suggestionsConfig.AutoContinue)
					}
				} else {
					hasWarnings = true
					l.Warn("", zap.Error(err))
					if findSuggestions {
						suggestions.FindSuggestion(err, suggestionToken, fileType, suggestionsConfig.AutoContinue)
					}
				}
			}

			uErr := errors.GetUnsupportedErr(err)
			if uErr != nil {
				hasWarnings = true
				l.Warn("", zap.Error(err))
			}
		}

		if hasErrors {
			status := "OpenAPI spec invalid ✖"
			github.GenerateSummary(status, errs)
			return fmt.Errorf(status)
		}
	}

	for _, warn := range g.GetWarnings() {
		hasWarnings = true
		if errs == nil {
			errs = []error{}
		}
		errs = append(errs, warn)
	}

	if hasWarnings {
		github.GenerateSummary("OpenAPI spec valid with warnings ⚠", errs)
		fmt.Printf("OpenAPI spec %s\n", utils.Yellow("valid with warnings ⚠"))
		return nil
	}

	github.GenerateSummary("OpenAPI spec valid ✓", nil)
	fmt.Printf("OpenAPI spec %s\n", utils.Green("valid ✓"))

	return nil
}
