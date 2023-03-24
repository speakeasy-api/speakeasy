package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/speakeasy-api/openapi-generation/v2/pkg/errors"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/logging"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	levelInfo = "info"
	levelWarn = "warn"
	levelErr  = "error"
)

type Logger struct {
	associatedFile string
	fields         []zapcore.Field
}

var _ logging.Logger = (*Logger)(nil)

func NewLogger(associatedFile string) *Logger {
	// If this is running via our action it will have a /repo/ prefix that needs to be removed to associate the file correctly
	associatedFile = strings.TrimPrefix(associatedFile, "/repo/")

	return &Logger{
		associatedFile: associatedFile,
	}
}

func (l *Logger) Info(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	fmt.Printf("%s%s%s\n", l.getPrefix(levelInfo, nil), msg, fieldsToJSON(fields))
}

func (l *Logger) Warn(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	msg, err, fields := getMessage(msg, fields)

	fmt.Printf("%s%s%s\n", l.getPrefix(levelWarn, err), msg, fieldsToJSON(fields))
}

func (l *Logger) Error(msg string, fields ...zapcore.Field) {
	fields = append(l.fields, fields...)

	msg, err, fields := getMessage(msg, fields)

	fmt.Fprintf(os.Stderr, utils.Red("%s%s%s\n"), l.getPrefix(levelErr, err), msg, fieldsToJSON(fields))
}

func (l *Logger) With(fields ...zapcore.Field) logging.Logger {
	return &Logger{
		associatedFile: l.associatedFile,
		fields:         append(l.fields, fields...),
	}
}

func (l *Logger) getPrefix(level string, err error) string {
	switch level {
	case levelInfo:
		return utils.Blue("INFO\t")
	case levelWarn:
		if env.IsGithubAction() {
			attributes := getGithubAnnotationAttributes(l.associatedFile, err)
			return fmt.Sprintf("::warning%s::", attributes)
		} else {
			return utils.Yellow("WARN\t")
		}
	case levelErr:
		if env.IsGithubAction() {
			attributes := getGithubAnnotationAttributes(l.associatedFile, err)
			return fmt.Sprintf("::error%s::", attributes)
		} else {
			return utils.Red("ERROR\t")
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
		return fmt.Sprintf(" file=%s,line=%d,title=Validation Error", filepath.Clean(associatedFile), vErr.LineNumber)
	}

	uErr := errors.GetUnsupportedErr(err)
	if uErr != nil {
		return fmt.Sprintf(" file=%s,line=%d,title=Unsupported", filepath.Clean(associatedFile), uErr.GetLineNumber())
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
