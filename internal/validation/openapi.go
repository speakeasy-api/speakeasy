package validation

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/speakeasy-api/openapi-generation/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func ValidateOpenAPI(ctx context.Context, schemaPath string) []error {
	fmt.Println("Validating OpenAPI spec...")

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return []error{fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)}
	}

	g, err := generate.New(generate.WithFileFuncs(func(filename string, data []byte, checkExisting bool) error { return nil }, func(filename string) ([]byte, error) { return nil, nil }), generate.WithLogger(log.Logger()))
	if err != nil {
		return []error{err}
	}

	if errs := g.Validate(context.Background(), schema); len(errs) > 0 {
		return errs
	}

	green := color.New(color.FgGreen).SprintFunc()

	fmt.Printf("OpenAPI spec %s\n", green("valid ✓"))

	return nil
}
