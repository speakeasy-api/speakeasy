package validation

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
)

func ValidateOpenAPI(ctx context.Context, schemaPath string) error {
	fmt.Println("Validating OpenAPI spec...")

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	l := log.Logger()

	g, err := generate.New(generate.WithFileFuncs(func(filename string, data []byte, perm os.FileMode) error { return nil }, func(filename string) ([]byte, error) { return nil, nil }), generate.WithLogger(l))
	if err != nil {
		return err
	}

	hasWarnings := false
	if errs := g.Validate(context.Background(), schema); len(errs) > 0 {
		hasErrors := false

		for _, err := range errs {
			vErr := errors.GetValidationErr(err)
			if vErr != nil {
				if vErr.Severity == errors.SeverityError {
					hasErrors = true
					l.Error(err.Error())
				} else {
					hasWarnings = true
					l.Warn(err.Error())
				}
			}
		}

		if hasErrors {
			return fmt.Errorf("OpenAPI spec invalid ✖")
		}
	}

	if hasWarnings {
		fmt.Printf("OpenAPI spec %s\n", utils.Yellow("valid with warnings"))
		return nil
	}

	fmt.Printf("OpenAPI spec %s\n", utils.Green("valid ✓"))
	return nil
}
