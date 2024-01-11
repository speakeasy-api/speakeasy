package log

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/logging"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level string

const (
	LevelInfo        Level = "info"
	LevelWarn        Level = "warn"
	LevelErr         Level = "error"
	loggerContextKey       = "cli-logger-context"
)

var Levels = []string{string(LevelInfo), string(LevelWarn), string(LevelErr)}

type Logger struct {
	level           Level
	associatedFile  string
	fields          []zapcore.Field
	interactiveOnly bool
	style           *lipgloss.Style
}

var _ logging.Logger = (*Logger)(nil)

// With returns a new context with the given logger added to the context.
func With(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// From returns the logger associated with the given context.
func From(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerContextKey).(Logger); ok {
		return l
	}
	return Logger{
		level: LevelInfo,
	}
}

/**
 * Builders
 */

func (l Logger) WithLevel(level Level) Logger {
	return Logger{
		level:          level,
		associatedFile: l.associatedFile,
		fields:         l.fields,
	}
}

func (l Logger) WithAssociatedFile(associatedFile string) Logger {
	// If this is running via our action it will have a /repo/ prefix that needs to be removed to associate the file correctly
	associatedFile = strings.TrimPrefix(associatedFile, "/repo/")

	return Logger{
		level:          l.level,
		associatedFile: associatedFile,
		fields:         l.fields,
	}
}

func (l Logger) WithInteractiveOnly() Logger {
	return Logger{
		level:           l.level,
		associatedFile:  l.associatedFile,
		fields:          l.fields,
		interactiveOnly: true,
	}
}

func (l Logger) WithStyle(style lipgloss.Style) Logger {
	return Logger{
		level:          l.level,
		associatedFile: l.associatedFile,
		fields:         l.fields,
		style:          &style,
	}
}

func (l Logger) With(fields ...zapcore.Field) logging.Logger {
	return &Logger{
		associatedFile: l.associatedFile,
		fields:         append(l.fields, fields...),
	}
}

/**
 * Logging methods
 */

func (l Logger) Info(msg string, fields ...zapcore.Field) {
	if l.level != LevelInfo {
		return
	}

	fields = append(l.fields, fields...)

	msg, err, fields := getMessage(msg, fields)

	msg = fmt.Sprintf("%s%s%s\n", l.getPrefix(LevelInfo, err), msg, fieldsToJSON(fields))
	fmt.Fprintf(os.Stderr, msg)
}

func (l Logger) Warn(msg string, fields ...zapcore.Field) {
	if l.level != LevelInfo && l.level != LevelWarn {
		return
	}

	fields = append(l.fields, fields...)

	msg, err, fields := getMessage(msg, fields)

	msg = fmt.Sprintf("%s%s%s\n", l.getPrefix(LevelWarn, err), msg, fieldsToJSON(fields))
	fmt.Fprintf(os.Stderr, msg)
}

func (l Logger) Error(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	msg, err, fields := getMessage(msg, fields)

	msg = fmt.Sprintf("%s%s%s\n", l.getPrefix(LevelErr, err), msg, fieldsToJSON(fields))
	fmt.Fprintf(os.Stderr, msg)
}

func (l Logger) Printf(format string, a ...any) {
	l.Println(fmt.Sprintf(format, a...))
}

func (l Logger) Println(s string) {
	if l.interactiveOnly && !utils.IsInteractive() {
		return
	}
	if l.style != nil {
		s = l.style.Render(s)
	}
	fmt.Fprintln(os.Stderr, s)
}

func (l Logger) PrintlnUnstyled(a any) {
	if l.interactiveOnly && !utils.IsInteractive() {
		return
	}
	fmt.Fprintln(os.Stderr, a)
}

/**
 * Utilities
 */

func (l Logger) getPrefix(level Level, err error) string {
	switch level {
	case LevelInfo:
		return styles.Info.Render("INFO\t")
	case LevelWarn:
		if env.IsGithubAction() {
			attributes := getGithubAnnotationAttributes(l.associatedFile, err)
			return fmt.Sprintf("::warning%s::", attributes)
		} else {
			return styles.Warning.Render("WARN\t")
		}
	case LevelErr:
		if env.IsGithubAction() {
			attributes := getGithubAnnotationAttributes(l.associatedFile, err)
			return fmt.Sprintf("::error%s::", attributes)
		} else {
			return styles.Error.Render("ERROR\t")
		}
	}

	return ""
}

func getGithubAnnotationAttributes(associatedFile string, err error) string {
	if err == nil {
		return ""
	}

	vErr := errors.GetValidationErr(err)
	if vErr != nil {
		severity := "Error"

		switch vErr.Severity {
		case errors.SeverityWarn:
			severity = "Warning"
		}

		return fmt.Sprintf(" file=%s,line=%d,title=Validation %s", filepath.Clean(associatedFile), vErr.LineNumber, severity)
	}

	uErr := errors.GetUnsupportedErr(err)
	if uErr != nil {
		return fmt.Sprintf(" file=%s,line=%d,title=Unsupported", filepath.Clean(associatedFile), uErr.LineNumber)
	}

	return ""
}

func getMessage(msg string, fields []zapcore.Field) (string, error, []zapcore.Field) {
	fields, err := findError(fields)
	if err != nil {
		if msg == "" {
			msg = err.Error()
		} else {
			fields = append(fields, zap.Error(err))
		}
	}

	return msg, err, fields
}

func findError(fields []zapcore.Field) ([]zapcore.Field, error) {
	var err error
	filteredFields := []zapcore.Field{}
	for _, field := range fields {
		if field.Type == zapcore.ErrorType {
			if foundErr, ok := field.Interface.(error); ok {
				err = foundErr
			} else {
				filteredFields = append(filteredFields, field)
			}
		} else {
			filteredFields = append(filteredFields, field)
		}
	}

	return filteredFields, err
}

func fieldsToJSON(fields []zapcore.Field) string {
	jsonObj := map[string]any{}

	for _, field := range fields {
		switch field.Type {
		case zapcore.StringType:
			jsonObj[field.Key] = field.String
		case zapcore.ErrorType:
			err, ok := field.Interface.(error)
			if !ok {
				jsonObj[field.Key] = field.Interface
			}
			jsonObj[field.Key] = err.Error()
		default:
			panic("not yet implemented")
		}
	}

	if len(jsonObj) == 0 {
		return ""
	}

	data, err := json.Marshal(jsonObj)
	if err != nil {
		return ""
	}

	return "\t" + string(data)
}
