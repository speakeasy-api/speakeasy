package arazzo

import (
	"context"
	"fmt"
	"os"

	"github.com/speakeasy-api/openapi/arazzo"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/log"
)

func Validate(ctx context.Context, file string) error {
	logger := log.From(ctx)
	logger.Info("Validating Arazzo document...\n")

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	_, validationErrors, err := arazzo.Unmarshal(ctx, f)
	if err != nil {
		return fmt.Errorf("failed to unmarshal file: %w", err)
	}

	if len(validationErrors) == 0 {
		msg := styles.RenderSuccessMessage(
			"Arazzo document valid ✓",
			"0 errors",
		)
		logger.Println(msg)
		return nil
	}

	lines := make([]string, len(validationErrors))

	for _, err := range validationErrors {
		lines = append(lines, fmt.Sprintf("- %s", err.Error()))
	}

	msg := styles.RenderErrorMessage("Validation Errors", lines...)
	logger.Println(msg)

	return fmt.Errorf(`Arazzo document invalid ✖`)
}
