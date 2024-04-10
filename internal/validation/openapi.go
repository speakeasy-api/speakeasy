package validation

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/generate"
	"github.com/speakeasy-api/sdk-gen-config/workspace"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/github"
	"github.com/speakeasy-api/speakeasy/internal/interactivity"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/schema"
	"go.uber.org/zap"
)

// OutputLimits defines the limits for validation output.
type OutputLimits struct {
	// MaxErrors prevents errors after this limit from being displayed.
	MaxErrors int

	// MaxWarns prevents warnings after this limit from being displayed.
	MaxWarns int
}

func ValidateWithInteractivity(ctx context.Context, schemaPath, header, token string, limits *OutputLimits, defaultRuleset, workingDir string) error {
	logger := log.From(ctx)
	logger.Info("Validating OpenAPI spec...\n")

	ctx = log.With(ctx, logger.WithWriter(io.Discard))

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	vErrs, vWarns, vInfo, err := Validate(ctx, logger, schema, schemaPath, limits, isRemote, defaultRuleset, workingDir)
	if err != nil {
		return err
	}

	if len(vErrs) == 0 && len(vWarns) == 0 && len(vInfo) == 0 {
		msg := styles.RenderSuccessMessage(
			"OpenAPI Document Valid",
			"0 errors, 0 warnings, 0 hints",
			fmt.Sprintf("Try %s %s", styles.HeavilyEmphasized.Render("speakeasy quickstart"), styles.Dimmed.Render("to generate an SDK")),
		)
		logger.Println(msg)
		return nil
	}

	var tabs []interactivity.Tab
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Errors (%d)", len(vErrs)),
		Content:     errorsToTabContents(schema, vErrs),
		TitleColor:  styles.Colors.Red,
		BorderColor: styles.Colors.Red,
		Default:     len(vErrs) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Warnings (%d)", len(vWarns)),
		Content:     errorsToTabContents(schema, vWarns),
		TitleColor:  styles.Colors.Yellow,
		BorderColor: styles.Colors.Yellow,
		Default:     len(vErrs) == 0 && len(vWarns) > 0,
	})
	tabs = append(tabs, interactivity.Tab{
		Title:       fmt.Sprintf("Hints (%d)", len(vInfo)),
		Content:     errorsToTabContents(schema, vInfo),
		TitleColor:  styles.Colors.Blue,
		BorderColor: styles.Colors.Blue,
		Default:     len(vErrs) == 0 && len(vWarns) == 0 && len(vInfo) > 0,
	})

	interactivity.RunTabs(tabs)

	return nil
}

func ValidateOpenAPI(ctx context.Context, schemaPath, header, token string, limits *OutputLimits, defaultRuleset, workingDir string) error {
	logger := log.From(ctx)
	logger.Info("Validating OpenAPI spec...\n")

	isRemote, schema, err := schema.GetSchemaContents(ctx, schemaPath, header, token)
	if err != nil {
		return fmt.Errorf("failed to get schema contents: %w", err)
	}

	prefixedLogger := logger.WithAssociatedFile(schemaPath).WithFormatter(log.PrefixedFormatter)

	hasWarnings := false

	vErrs, vWarns, vInfo, err := Validate(ctx, logger, schema, schemaPath, limits, isRemote, defaultRuleset, workingDir)
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

	logger.Infof("\nOpenAPI spec validation complete. %d errors, %d warnings, %d hints\n", len(vErrs), len(vWarns), len(vInfo))

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

	if err.LineNumber == -1 {
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
	if endLine > len(lines) {
		endLine = len(lines)
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
func Validate(ctx context.Context, outputLogger log.Logger, schema []byte, schemaPath string, limits *OutputLimits, isRemote bool, defaultRuleset, workingDir string) ([]error, []error, []error, error) {
	l := log.From(ctx).WithFormatter(log.PrefixedFormatter)

	opts := []generate.GeneratorOptions{
		generate.WithDontWrite(),
		generate.WithLogger(l),
		generate.WithRunLocation("cli"),
	}

	if defaultRuleset != "" {
		opts = append(opts, generate.WithValidationRuleset(defaultRuleset))
	}

	g, err := generate.New(opts...)
	if err != nil {
		return nil, nil, nil, err
	}

	res, err := g.Validate(ctx, schema, schemaPath, isRemote, workingDir)
	if err != nil {
		return nil, nil, nil, err
	}
	var vErrs []error
	var vWarns []error
	var vInfo []error

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

	searchDirs := map[string]bool{
		workingDir: true,
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		searchDirs[homeDir] = false
	}

	outputDir := filepath.Join(os.TempDir(), "speakeasy")

	for dir, allowRecursive := range searchDirs {
		res, err := workspace.FindWorkspace(dir, workspace.FindWorkspaceOptions{
			Recursive: allowRecursive,
		})
		if err != nil {
			continue
		}
		outputDir = filepath.Join(res.Path, "temp")
	}

	_ = os.MkdirAll(outputDir, 0o755)

	rf, err := os.CreateTemp(outputDir, "lint-report-*.html")
	if err == nil {
		defer rf.Close()

		if _, err := rf.Write(res.GenerateReport()); err == nil {
			outputLogger.Info(fmt.Sprintf("Validation report written to: %s", rf.Name()))
		}
	}

	return vErrs, vWarns, vInfo, nil
}
