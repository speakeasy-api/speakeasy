package validation

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
	"os"
)

func ValidateOpenAPI(ctx context.Context, schemaPath string, findSuggestions bool) error {
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
		suggestionToken := ""

		if findSuggestions {
			suggestionToken, err = suggestions.Upload(schemaPath)
			if err != nil {
				l.Error("cannot fetch llm suggestions due to error", zap.Error(err))
				findSuggestions = false
			}
		}

		for _, err := range errs {
			vErr := errors.GetValidationErr(err)
			if vErr != nil {
				if vErr.Severity == errors.SeverityError {
					hasErrors = true
					l.Error("", zap.Error(err))
					if findSuggestions {
						suggestions.FindSuggestion(err, suggestionToken)
					}
				} else {
					hasWarnings = true
					l.Warn("", zap.Error(err))
				}
			}

			uErr := errors.GetUnsupportedErr(err)
			if uErr != nil {
				hasWarnings = true
				l.Warn("", zap.Error(err))
			}
		}

		if findSuggestions {
			suggestions.Clear(suggestionToken)
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
