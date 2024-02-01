package run

import (
	"bytes"
	"github.com/speakeasy-api/openapi-generation/v2/pkg/logging"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogInterceptor struct {
	logger log.Logger
}

func (l LogInterceptor) Warn(msg string, fields ...zapcore.Field) {
	//TODO implement me
	panic("implement me")
}

func (l LogInterceptor) Error(msg string, fields ...zapcore.Field) {
	//TODO implement me
	panic("implement me")
}

func (l LogInterceptor) With(fields ...zapcore.Field) logging.Logger {
	//TODO implement me
	panic("implement me")
}

func NewLogInterceptor(logger log.Logger, out bytes.Buffer) logging.Logger {
	return LogInterceptor{logger: logger.WithWriter(&out)}
}

func (l LogInterceptor) Info(msg string, fields ...zap.Field) {
	//TODO implement me
	panic("implement me")
}

func (l LogInterceptor) Github(msg string) {
	//TODO implement me
	panic("implement me")
}

var _ logging.Logger = (*LogInterceptor)(nil)
