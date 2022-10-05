package validation

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/speakeasy-api/openapi-generation/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func ValidateOpenAPI(ctx context.Context, schemaPath string) error {
	fmt.Println("Validating OpenAPI spec...")

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	g, err := generate.New(generate.WithOnWriteFileFunc(func(filename string, data []byte) error { return nil }), generate.WithLogger(log.Logger()))
	if err != nil {
		return err
	}

	if err := g.Validate(context.Background(), schema); err != nil {
		return errors.New(generate.GetOrderedErrorString(err))
	}

	green := color.New(color.FgGreen).SprintFunc()

	fmt.Printf("OpenAPI spec %s\n", green("valid ✓"))

	return nil
}
