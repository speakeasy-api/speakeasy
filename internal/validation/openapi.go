package validation

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/logging"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
	"os"
)

func ValidateOpenAPI(ctx context.Context, schemaPath string, outputHints bool) error {
	fmt.Println("Validating OpenAPI spec...")
	fmt.Println()

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	l := log.NewLogger(schemaPath)

	hasWarnings := false

	vErrs, vWarns, vInfo, err := Validate(schema, schemaPath, outputHints)
	if err != nil {
		return err
	}

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

	if len(vErrs) > 0 {
		status := "OpenAPI spec invalid ✖"
		github.GenerateSummary(status, vErrs)
		return fmt.Errorf(status)
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
	// Set to error because g.Validate sometimes logs all warnings for some reason
	l := logging.NewLogger(zap.ErrorLevel)

	g, err := generate.New(generate.WithFileFuncs(
		func(filename string, data []byte, perm os.FileMode) error { return nil },
		func(filename string) ([]byte, error) { return nil, nil },
	), generate.WithLogger(l))

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
