package validation

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/speakeasy-api/sdk-gen-config/lint"

	"github.com/speakeasy-api/speakeasy-core/openapi"

	"github.com/speakeasy-api/speakeasy/internal/reports"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/speakeasy-client-sdk-go/v3/pkg/models/shared"
	"github.com/speakeasy-api/speakeasy-core/events"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"go.uber.org/zap"
)

// OutputLimits defines the limits for validation output.
type OutputLimits struct {
	// MaxErrors prevents errors after this limit from being displayed.
	MaxErrors int

	// MaxWarns prevents warnings after this limit from being displayed.
	MaxWarns int
}

type ValidationResult struct {
	AllErrors         []error
	Errors            []error
	Warnings          []error
	Infos             []error
	Status            string
	ValidOperations   []string
	InvalidOperations []string
	Report            *reports.ReportResult
}

var validSpeakeasyRulesets = []string{"speakeasy-recommended", "speakeasy-generation", "speakeasy-openapi", "vacuum", "owasp"}

func ValidateWithInteractivity(ctx context.Context, schemaPath, header, token string, limits *OutputLimits, defaultRuleset, workingDir string, skipGenerateReport bool) (*ValidationResult, error) {
	logger := log.From(ctx)
	logger.Info("Linting OpenAPI document...\n")

	ctx = log.With(ctx, logger.WithWriter(io.Discard))

	isRemote, schema, err := openapi.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get document contents: %w", err)
	}

	res, err := Validate(ctx, logger, schema, schemaPath, limits, isRemote, defaultRuleset, workingDir, false, skipGenerateReport)
	if err != nil {
		return nil, err
	}

	if len(res.Errors) == 0 && len(res.Warnings) == 0 && len(res.Infos) == 0 {
		msg := styles.RenderSuccessMessage(
			"OpenAPI Document Valid",
			"0 errors, 0 warnings, 0 hints",
			fmt.Sprintf("Try %s %s", styles.HeavilyEmphasized.Render("speakeasy quickstart"), styles.Dimmed.Render("to generate an SDK")),
		)
		logger.Println(msg)
		return res, nil
	}

	var tabs []interactivity.Tab
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Errors (%d)", len(res.Errors)),
		Content:     errorsToTabContents(schema, res.Errors),
		TitleColor:  styles.Colors.Red,
		BorderColor: styles.Colors.Red,
		Default:     len(res.Errors) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Warnings (%d)", len(res.Warnings)),
		Content:     errorsToTabContents(schema, res.Warnings),
		TitleColor:  styles.Colors.Yellow,
		BorderColor: styles.Colors.Yellow,
		Default:     len(res.Errors) == 0 && len(res.Warnings) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Hints (%d)", len(res.Infos)),
		Content:     errorsToTabContents(schema, res.Infos),
		TitleColor:  styles.Colors.Blue,
		BorderColor: styles.Colors.Blue,
		Default:     len(res.Errors) == 0 && len(res.Warnings) == 0 && len(res.Infos) > 0,
	})

	interactivity.RunTabs(tabs)

	return res, nil
}

func ValidateOpenAPI(ctx context.Context, source, schemaPath, header, token string, limits *OutputLimits, defaultRuleset, workingDir string, isQuickstart bool, skipGenerateReport bool) (*ValidationResult, error) {
	logger := log.From(ctx)
	logger.Info("Linting OpenAPI document...\n")

	isRemote, schema, err := openapi.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema contents: %w", err)
	}

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	res, err := Validate(ctx, logger, schema, schemaPath, limits, isRemote, defaultRuleset, workingDir, isQuickstart, skipGenerateReport)
	if err != nil {
		return nil, err
	}

	for _, hint := range res.Infos {
		prefixedLogger.Info("", zap.Error(hint))
	}
	for _, warn := range res.Warnings {
		prefixedLogger.Warn("", zap.Error(warn))
	}
	for _, err := range res.Errors {
		prefixedLogger.Error("", zap.Error(err))
	}

	logger.Infof("\nOpenAPI document linting complete. %d errors, %d warnings, %d hints\n", len(res.Errors), len(res.Warnings), len(res.Infos))

	reportURL := ""
	if res.Report != nil {
		reportURL = res.Report.URL
	}
	github.GenerateLintingSummary(ctx, github.LintingSummary{
		Source:    source,
		Status:    res.Status,
		Errors:    res.AllErrors,
		ReportURL: reportURL,
	})

	if len(res.Errors) > 0 {
		return res, errors.New(res.Status)
	}

	if len(res.Warnings) > 0 {
		logger.Warn(res.Status)
		return res, nil
	}

	logger.Success(res.Status)
	return res, nil
}

func errorsToTabContents(schema []byte, errs []error) []interactivity.InspectableContent {
	var contents []interactivity.InspectableContent

	lines := strings.Split(string(schema), "\n")

	// Truncate very long lines (common for single-line json specs)
	for i, line := range lines {
		if len(line) > 1000 {
			lines[i] = line[:1000] + "..."
		}
	}

	for _, err := range errs {
		vErr := errors.GetValidationErr(err)

		s := ""
		var details *string

		// Need to account for non-validation errors
		if vErr == nil {
			s = fmt.Sprintf("%v", err)
		} else {
			lineNumber := styles.SeverityToStyle(vErr.Severity).Render(fmt.Sprintf("Line %d:", vErr.LineNumber))
			errType := styles.Dimmed.Render(vErr.Rule)
			s = fmt.Sprintf("%s %s - %s", lineNumber, errType, vErr.Message)
			d := getDetailedView(lines, *vErr)
			details = &d
		}

		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: details,
		}

		contents = append(contents, content)
	}

	if len(errs) == 0 {
		s := styles.Emphasized.Render("Congrats, there are no issues!")
		content := interactivity.InspectableContent{
			Summary:      s,
			DetailedView: nil,
		}
		contents = append(contents, content)
	}

	return contents
}

func getDetailedView(lines []string, err errors.ValidationError) string {
	var sb strings.Builder

	errAndLine := styles.SeverityToStyle(err.Severity).Render(fmt.Sprintf("%s on line %d", err.Severity, err.LineNumber))
	sb.WriteString(fmt.Sprintf("%s %s\n", errAndLine, styles.Dimmed.Render(err.Rule)))
	sb.WriteString(err.Message)
	sb.WriteString("\n\n")

	if err.LineNumber < 0 || err.LineNumber > len(lines)-1 {
		sb.WriteString(styles.Dimmed.Render("This error does not apply to any specific line."))
		return sb.String()
	}

	sb.WriteString(styles.Emphasized.Render("Surrounding Lines:"))
	sb.WriteString("\n")

	startLine := err.LineNumber - 4
	if startLine < 0 {
		startLine = 0
	}

	endLine := err.LineNumber + 3
	if endLine > len(lines)-1 {
		endLine = len(lines) - 1
	}

	shortestWhitespacePrefix := ""
	for i, line := range lines[startLine:endLine] {
		trimmed := strings.TrimLeft(line, " ")
		prefixLen := len(line) - len(trimmed)
		if i == 0 || prefixLen < len(shortestWhitespacePrefix) {
			shortestWhitespacePrefix = strings.Repeat(" ", prefixLen)
		}
	}

	for i, line := range lines[startLine:endLine] {
		lineNumber := startLine + i + 1
		lineNumString := styles.Dimmed.Render(fmt.Sprintf("%d", lineNumber))
		if lineNumber == err.LineNumber {
			lineNumString = styles.Error.Render(fmt.Sprintf("%d", lineNumber))
		}

		trimmedContent := strings.TrimPrefix(line, shortestWhitespacePrefix)

		sb.WriteString(fmt.Sprintf("%s %s\n", lineNumString, trimmedContent))
	}

	return sb.String()
}

// Validate returns (validation errors, validation warnings, validation info, error)
func Validate(ctx context.Context, outputLogger log.Logger, schema []byte, schemaPath string, limits *OutputLimits, isRemote bool, defaultRuleset, workingDir string, parseValidOperations bool, skipGenerateReport bool) (*ValidationResult, error) {
	l := log.From(ctx).WithFormatter(log.PrefixedFormatter)

	opts := []generate.GeneratorOptions{
		generate.WithDontWrite(),
		generate.WithLogger(l),
		generate.WithRunLocation("cli"),
	}

	if parseValidOperations {
		opts = append(opts, generate.WithParseValidOperations())
	}

	if defaultRuleset != "" {
		if !slices.Contains(validSpeakeasyRulesets, defaultRuleset) {
			lintConfig, _, err := lint.Load([]string{"."})
			if err != nil {
				return nil, fmt.Errorf("failed to load .speakeasy/lint.yaml: %w", err)
			}

			if _, ok := lintConfig.Rulesets[defaultRuleset]; !ok {
				return nil, fmt.Errorf("specified ruleset %s not found in .speakeasy/lint.yaml", defaultRuleset)
			}
		}

		opts = append(opts, generate.WithValidationRuleset(defaultRuleset))
	}

	g, err := generate.New(opts...)
	if err != nil {
		return nil, err
	}

	res, err := g.Validate(ctx, schema, schemaPath, isRemote, workingDir)
	if err != nil {
		return nil, err
	}
	var vErrs, vWarns, vInfo []error

	errs := res.GetValidationErrors()
	for _, err := range errs {
		vErr := errors.GetValidationErr(err)
		uErr := errors.GetUnsupportedErr(err)

		switch {
		case vErr != nil:
			if vErr.Severity == errors.SeverityError {
				vErrs = append(vErrs, vErr)
			} else if vErr.Severity == errors.SeverityWarn {
				vWarns = append(vWarns, vErr)
			} else {
				vInfo = append(vInfo, vErr)
			}
		case uErr != nil:
			vWarns = append(vWarns, uErr)
		default:
			vErrs = append(vErrs, err)
		}
	}

	vWarns = append(vWarns, g.GetWarnings()...)

	if limits != nil {
		if limits.MaxWarns > 0 && len(vWarns) > limits.MaxWarns {
			vWarns = append(vWarns, fmt.Errorf("and %d more warnings", len(vWarns)-limits.MaxWarns+1))
			vWarns = vWarns[:limits.MaxWarns-1]
		}

		if limits.MaxErrors > 0 && len(vErrs) > limits.MaxErrors {
			vErrs = append(vErrs, fmt.Errorf("and %d more errors", len(vWarns)-limits.MaxErrors+1))
			vErrs = vErrs[:limits.MaxErrors]
		}
	}

	status := "OpenAPI document valid ✓"

	if len(vErrs) > 0 {
		status = "OpenAPI document invalid ✖"
	} else if len(vWarns) > 0 {
		status = "OpenAPI document valid with warnings ⚠"
	}

	var report *reports.ReportResult
	if !utils.IsZeroTelemetryOrganization(ctx) && !skipGenerateReport {
		resultReport, err := generateReport(ctx, res)
		if err == nil && resultReport.Message != "" {
			outputLogger.Info(resultReport.Message)
		}
		report = &resultReport
	}

	cliEvent := events.GetTelemetryEventFromContext(ctx)
	if cliEvent != nil {
		infoCount := int64(len(vInfo))
		warnCount := int64(len(vWarns))
		errCount := int64(len(vErrs))
		cliEvent.LintReportInfoCount = &infoCount
		cliEvent.LintReportWarningCount = &warnCount
		cliEvent.LintReportErrorCount = &errCount
	}

	return &ValidationResult{
		AllErrors:         errs,
		Errors:            vErrs,
		Warnings:          vWarns,
		Infos:             vInfo,
		Status:            status,
		ValidOperations:   res.GetValidOperations(),
		InvalidOperations: res.GetInvalidOperations(),
		Report:            report,
	}, nil
}

type validationResult interface {
	GenerateReport() []byte
}

// Returns (message, url, digest, error)
func generateReport(ctx context.Context, res validationResult) (reports.ReportResult, error) {
	reportBytes := res.GenerateReport()
	return reports.UploadReport(ctx, reportBytes, shared.TypeLinting)
}
