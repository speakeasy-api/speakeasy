package validation

import (
	"context"
	"fmt"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"go.uber.org/zap"
	"io"
	"os"
)

// OutputLimits defines the limits for validation output.
type OutputLimits struct {
	// MaxErrors prevents errors after this limit from being displayed.
	MaxErrors int

	// MaxWarns prevents warnings after this limit from being displayed.
	MaxWarns int

	// OutputHints enables hints to be displayed.
	OutputHints bool
}

func ValidateWithInteractivity(ctx context.Context, schemaPath, header, token string, limits *OutputLimits) error {
	logger := log.From(ctx)
	logger.Info("Validating OpenAPI spec...\n")

	ctx = log.With(ctx, logger.WithWriter(io.Discard))

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	vErrs, vWarns, vInfo, err := Validate(ctx, schema, schemaPath, limits, isRemote)
	if err != nil {
		return err
	}

	var tabs []interactivity.Tab
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Errors (%d)", len(vErrs)),
		Content:     errorsToStrings(vErrs),
		TitleColor:  styles.Colors.Red,
		BorderColor: styles.Colors.Red,
		Default:     len(vErrs) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Warnings (%d)", len(vWarns)),
		Content:     errorsToStrings(vWarns),
		TitleColor:  styles.Colors.Yellow,
		BorderColor: styles.Colors.Yellow,
		Default:     len(vErrs) == 0 && len(vWarns) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Hints (%d)", len(vInfo)),
		Content:     errorsToStrings(vInfo),
		TitleColor:  styles.Colors.Blue,
		BorderColor: styles.Colors.Blue,
		Default:     len(vErrs) == 0 && len(vWarns) == 0 && len(vInfo) > 0,
	})

	interactivity.RunTabs(tabs)

	return nil
}

func ValidateOpenAPI(ctx context.Context, schemaPath, header, token string, limits *OutputLimits) error {
	logger := log.From(ctx)
	logger.Info("Validating OpenAPI spec...\n")

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	hasWarnings := false

	vErrs, vWarns, vInfo, err := Validate(ctx, schema, schemaPath, limits, isRemote)
	if err != nil {
		return err
	}

	for _, hint := range vInfo {
		prefixedLogger.Info("", zap.Error(hint))
	}
	for _, warn := range vWarns {
		hasWarnings = true
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range vErrs {
		prefixedLogger.Error("", zap.Error(err))
	}

	logger.Infof("OpenAPI spec validation complete. %d errors, %d warnings, %d hints\n", len(vErrs), len(vWarns), len(vInfo))

	if len(vErrs) > 0 {
		status := "\nOpenAPI spec invalid ✖"
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
		logger.Warn("OpenAPI spec valid with warnings ⚠")
		return nil
	}

	github.GenerateSummary("OpenAPI spec valid ✓", nil)
	logger.Success("OpenAPI spec valid ✓")

	return nil
}

func errorsToStrings(errs []error) []string {
	var strings []string
	for _, err := range errs {
		vErr := errors.GetValidationErr(err)

		style := styles.Info
		switch vErr.Severity {
		case errors.SeverityError:
			style = styles.Error
		case errors.SeverityWarn:
			style = styles.Warning
		case errors.SeverityHint:
			style = styles.Info
		}

		lineNumber := style.Render(fmt.Sprintf("Line %d:", vErr.LineNumber))
		errType := styles.Dimmed.Render(vErr.Rule)
		s := fmt.Sprintf("%s %s - %s", lineNumber, errType, vErr.Message)
		strings = append(strings, s)
	}
	if len(errs) == 0 {
		strings = append(strings, styles.Success.Render("Congrats, there are no issues!"))
	}
	return strings
}

// Validate returns (validation errors, validation warnings, validation info, error)
func Validate(ctx context.Context, schema []byte, schemaPath string, limits *OutputLimits, isRemote bool) ([]error, []error, []error, error) {
	// TODO: is this still true: Set to error because g.Validate sometimes logs all warnings for some reason
	l := log.From(ctx).WithFormatter(log.PrefixedFormatter)

	g, err := generate.New(generate.WithFileFuncs(
		func(filename string, data []byte, perm os.FileMode) error { return nil },
		func(filename string) ([]byte, error) { return nil, nil },
	), generate.WithLogger(l))

	if err != nil {
		return nil, nil, nil, err
	}

	errs := g.Validate(context.Background(), schema, schemaPath, limits.OutputHints, isRemote)
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

	if limits.MaxWarns > 0 && len(vWarns) > limits.MaxWarns {
		vWarns = append(vWarns, fmt.Errorf("and %d more warnings", len(vWarns)-limits.MaxWarns+1))
		vWarns = vWarns[:limits.MaxWarns-1]
	}

	if limits.MaxErrors > 0 && len(vErrs) > limits.MaxErrors {
		vErrs = append(vErrs, fmt.Errorf("and %d more errors", len(vWarns)-limits.MaxErrors+1))
		vErrs = vErrs[:limits.MaxErrors]
	}

	return vErrs, vWarns, vInfo, nil
}
