package log

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level string

type LoggerContextKey string

const (
	LevelDebug       Level            = "debug"
	LevelInfo        Level            = "info"
	LevelWarn        Level            = "warn"
	LevelErr         Level            = "error"
	LevelSuccess     Level            = "success"
	loggerContextKey LoggerContextKey = "cli-logger-context"
)

var Levels = []string{string(LevelInfo), string(LevelWarn), string(LevelErr)}

type Logger struct {
	level           Level
	associatedFile  string
	fields          []zapcore.Field
	interactiveOnly bool
	style           *lipgloss.Style
	formatter       func(l Logger, level Level, msg string, err error) string
	writer          io.Writer
	warnCapture     *[]string
	listener        chan Msg
	pathPrefix      []string // For Scope() support - prefix for StartStep paths
}

var _ logging.Logger = (*Logger)(nil)

// With returns a new context with the given logger added to the context.
func With(ctx context.Context, l logging.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// From returns the logger associated with the given context.
func From(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerContextKey).(Logger); ok {
		return l
	}
	return New()
}

func New() Logger {
	formatter := BasicFormatter

	if env.IsGithubAction() {
		formatter = GithubFormatter
	}

	return Logger{
		level:     LevelInfo,
		formatter: formatter,
		writer:    os.Stderr,
	}
}

func NewNoop() Logger {
	return Logger{
		level:     LevelErr,
		formatter: BasicFormatter,
		writer:    io.Discard,
	}
}

/**
 * Builders
 */

func (l Logger) WithLevel(level Level) Logger {
	l2 := l.Copy()
	l2.level = level
	return l2
}

func (l Logger) WithAssociatedFile(associatedFile string) Logger {
	// If this is running via our action it will have a /repo/ prefix that needs to be removed to associate the file correctly
	associatedFile = strings.TrimPrefix(associatedFile, "/repo/")

	l2 := l.Copy()
	l2.associatedFile = associatedFile
	return l2
}

func (l Logger) WithInteractiveOnly() Logger {
	l2 := l.Copy()
	l2.interactiveOnly = true
	return l2
}

func (l Logger) WithStyle(style lipgloss.Style) Logger {
	l2 := l.Copy()
	l2.style = &style
	return l2
}

func (l Logger) With(fields ...zapcore.Field) logging.Logger {
	l2 := l.Copy()
	l2.fields = append(l2.fields, fields...)
	return l2
}

func (l Logger) WithFormatter(formatter func(l Logger, level Level, msg string, err error) string) Logger {
	l2 := l.Copy()
	l2.formatter = formatter
	return l2
}

func (l Logger) WithWriter(w io.Writer) Logger {
	l2 := l.Copy()
	l2.writer = w
	return l2
}

func (l Logger) WithWarnCapture(dst *[]string) Logger {
	l2 := l.Copy()
	l2.warnCapture = dst
	return l2
}

func (l Logger) WithListener(listener chan Msg) Logger {
	l2 := l.Copy()
	l2.listener = listener
	return l2
}

func (l Logger) Copy() Logger {
	return Logger{
		level:           l.level,
		associatedFile:  l.associatedFile,
		fields:          l.fields,
		interactiveOnly: l.interactiveOnly,
		style:           l.style,
		formatter:       l.formatter,
		writer:          l.writer,
		warnCapture:     l.warnCapture,
		listener:        l.listener,
		pathPrefix:      l.pathPrefix,
	}
}

/**
 * Logging methods
 */

func (l Logger) Debug(msg string, fields ...zapcore.Field) {
	if l.level != LevelDebug {
		return
	}

	fields = append(l.fields, fields...)

	msg, fields, err := getMessage(msg, fields)

	msg = l.format(LevelDebug, msg, err) + fieldsToJSON(fields)
	l.Println(msg)
}

func (l Logger) Info(msg string, fields ...zapcore.Field) {
	if l.level != LevelInfo {
		return
	}

	fields = append(l.fields, fields...)

	msg, fields, err := getMessage(msg, fields)

	msg = l.format(LevelInfo, msg, err) + fieldsToJSON(fields)
	l.Println(msg)
}

func (l Logger) Infof(format string, a ...any) {
	l.Info(fmt.Sprintf(format, a...))
}

func (l Logger) Warn(msg string, fields ...zapcore.Field) {
	if l.level != LevelInfo && l.level != LevelWarn {
		return
	}

	fields = append(l.fields, fields...)

	msg, fields, err := getMessage(msg, fields)

	msg = l.format(LevelWarn, msg, err) + fieldsToJSON(fields)

	l.Println(msg)

	if l.warnCapture != nil {
		*l.warnCapture = append(*l.warnCapture, msg)
	}
}

func (l Logger) Warnf(format string, a ...any) {
	l.Warn(fmt.Sprintf(format, a...))
}

func (l Logger) Error(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	msg, fields, err := getMessage(msg, fields)

	msg = l.format(LevelErr, msg, err) + fieldsToJSON(fields)
	l.Println(msg)
}

func (l Logger) Errorf(format string, a ...any) {
	l.Error(fmt.Sprintf(format, a...))
}

func (l Logger) Success(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	msg, fields, err := getMessage(msg, fields)

	msg = l.format(LevelSuccess, msg, err) + fieldsToJSON(fields)
	l.Println(msg)
}

func (l Logger) Successf(format string, a ...any) {
	l.Success(fmt.Sprintf(format, a...))
}

func (l Logger) Printf(format string, a ...any) {
	l.Println(fmt.Sprintf(format, a...))
}

func (l Logger) PrintfStyled(style lipgloss.Style, format string, a ...any) {
	l.Fprintln(l.writer, style.Render(fmt.Sprintf(format, a...)))
}

func (l Logger) Println(s string) {
	l.Fprintln(l.writer, s)
}

func (l Logger) Fprintln(w io.Writer, s string) {
	l.Fprint(w, s+"\n")
}

func (l Logger) Print(s string) {
	l.Fprint(l.writer, s)
}

func (l Logger) PrintStyled(style lipgloss.Style, s string) {
	l.Fprintln(l.writer, style.Render(s))
}

func (l Logger) Fprint(w io.Writer, s string) {
	if w == nil {
		return
	}
	if l.interactiveOnly && (!utils.IsInteractive() || env.IsGithubAction()) {
		return
	}
	if l.style != nil {
		s = l.style.Render(s)
	}

	s = styles.InjectMarkdownStyles(s)

	_, _ = fmt.Fprint(w, s)
}

func (l Logger) PrintlnUnstyled(a any) {
	if l.interactiveOnly && !utils.IsInteractive() {
		return
	}
	_, _ = fmt.Fprintln(l.writer, a)
}

func (l Logger) format(level Level, msg string, err error) string {
	return l.formatter(l, level, msg, err)
}

func (l Logger) Github(msg string) {
	if l.listener != nil {
		msgType := MsgGithub
		cleanMsg := msg

		// Normalize message to handle trailing whitespace/newlines from external libraries
		trimmedMsg := strings.TrimSpace(msg)

		// Detect "(skipped)" suffix on group messages and promote to richer type
		if strings.HasPrefix(trimmedMsg, "::group::") && strings.HasSuffix(trimmedMsg, "(skipped)") {
			msgType = MsgStepSkipped
			// Remove suffix and any space before it
			cleanMsg = strings.TrimSuffix(trimmedMsg, "(skipped)")
			cleanMsg = strings.TrimSpace(cleanMsg)
		}

		l.listener <- Msg{Type: msgType, Msg: cleanMsg}
	}

	if env.IsGithubAction() {
		l.Print(msg)
	}
}

// Scope returns a Logger with the given name added to the path prefix.
// All StartStep calls on the returned logger will have this prefix in their path.
// Scopes can be nested: logger.Scope("A").Scope("B").StartStep("C") produces path ["A", "B", "C"].
func (l Logger) Scope(name string) logging.Logger {
	l2 := l.Copy()
	// Deep copy to prevent shared state issues between scopes
	l2.pathPrefix = make([]string, len(l.pathPrefix)+1)
	copy(l2.pathPrefix, l.pathPrefix)
	l2.pathPrefix[len(l.pathPrefix)] = name
	return l2
}

// StartStep starts a trackable progress step that appears in the workflow UI.
// The step is initially "pending" and can be updated via Succeed/Fail/Skip.
// Path is formed by: [scope prefix...] + msg
func (l Logger) StartStep(msg string) logging.Step {
	if l.listener == nil {
		return &noopStep{}
	}

	// Build full path from scope prefix + message
	path := make([]string, len(l.pathPrefix)+1)
	copy(path, l.pathPrefix)
	path[len(l.pathPrefix)] = msg

	id := uuid.NewString()
	l.listener <- Msg{
		Type: MsgStep,
		Step: &StepMsg{ID: id, Path: path, Status: StepStatusPending},
	}

	return &loggerStep{id: id, path: path, listener: l.listener}
}

// loggerStep implements logging.Step for real step tracking.
type loggerStep struct {
	id       string
	path     []string
	listener chan Msg
	once     sync.Once
}

func (s *loggerStep) Succeed() {
	s.once.Do(func() {
		s.listener <- Msg{Type: MsgStep, Step: &StepMsg{ID: s.id, Path: s.path, Status: StepStatusSuccess}}
	})
}

func (s *loggerStep) Fail() {
	s.once.Do(func() {
		s.listener <- Msg{Type: MsgStep, Step: &StepMsg{ID: s.id, Path: s.path, Status: StepStatusFailed}}
	})
}

func (s *loggerStep) Skip() {
	s.once.Do(func() {
		s.listener <- Msg{Type: MsgStep, Step: &StepMsg{ID: s.id, Path: s.path, Status: StepStatusSkipped}}
	})
}

// noopStep is a no-op implementation of logging.Step.
type noopStep struct{}

func (s *noopStep) Succeed() {}
func (s *noopStep) Fail()    {}
func (s *noopStep) Skip()    {}

/**
 * Formatters
 */

func BasicFormatter(l Logger, level Level, msg string, err error) string {
	switch level {
	case LevelDebug:
		return styles.Dimmed.Render(msg)
	case LevelInfo:
		return styles.Info.Render(msg)
	case LevelWarn:
		return styles.Warning.Render(msg)
	case LevelErr:
		return styles.Error.Render(msg)
	case LevelSuccess:
		return styles.Success.Render(msg)
	}

	return ""
}

func PrefixedFormatter(l Logger, level Level, msg string, err error) string {
	prefix := ""

	switch level {
	case LevelDebug:
		prefix = styles.Dimmed.Bold(true).Render("DEBUG\t")
	case LevelInfo, LevelSuccess:
		prefix = styles.Info.Bold(true).Render("INFO\t")
	case LevelWarn:
		prefix = styles.Warning.Bold(true).Render("WARN\t")
	case LevelErr:
		prefix = styles.Error.Bold(true).Render("ERROR\t")
	}

	return prefix + msg
}

func GithubFormatter(l Logger, level Level, msg string, err error) string {
	prefix := ""

	switch level {
	case LevelDebug:
		prefix = styles.Dimmed.Render("DEBUG\t")
	case LevelInfo:
		prefix = styles.Info.Render("INFO\t")
	case LevelSuccess:
		prefix = styles.Success.Render("SUCCESS\t")
	case LevelWarn:
		attributes := getGithubAnnotationAttributes(l.associatedFile, err)
		prefix = fmt.Sprintf("::warning%s::", attributes)
	case LevelErr:
		attributes := getGithubAnnotationAttributes(l.associatedFile, err)
		prefix = fmt.Sprintf("::error%s::", attributes)
	}

	return prefix + msg
}

/**
 * Utilities
 */

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
		case errors.SeverityError:
			severity = "Error"
		case errors.SeverityHint:
			severity = "Hint"
		}

		return fmt.Sprintf(" file=%s,line=%d,col=%d,title=Validation %s", filepath.Clean(associatedFile), vErr.GetLineNumber(), vErr.GetColumnNumber(), severity)
	}

	uErr := errors.GetUnsupportedErr(err)
	if uErr != nil {
		return fmt.Sprintf(" file=%s,line=%d,col=%d,title=Unsupported", filepath.Clean(associatedFile), uErr.GetLineNumber(), uErr.GetColumnNumber())
	}

	return ""
}

func getMessage(msg string, fields []zapcore.Field) (string, []zapcore.Field, error) {
	fields, err := findError(fields)
	if err != nil {
		if msg == "" {
			msg = err.Error()
		} else {
			fields = append(fields, zap.Error(err))
		}
	}

	return msg, fields, err
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
			bs, err := json.Marshal(field.Interface)
			if err != nil {
				jsonObj[field.Key] = "<skipped>"
				continue
			}
			jsonObj[field.Key] = json.RawMessage(bs)
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
