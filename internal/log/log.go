package log

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once      sync.Once
	zapLogger *zap.Logger
	config    *zap.Config
)

func Logger() *zap.Logger {
	once.Do(func() {
		if zapLogger = zap.L(); isNopLogger(zapLogger) {
			config = createConfig()
			var err error
			zapLogger, err = config.Build()
			if err != nil {
				fmt.Printf("Logger init failed with error: %s\n", err.Error())
				zapLogger = zap.NewNop()
			}
		}
	})

	return zapLogger
}

func createConfig() *zap.Config {
	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	return &zap.Config{
		Level:    level,
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			LevelKey:      "level",
			TimeKey:       "time",
			NameKey:       "name",
			CallerKey:     "caller",
			StacktraceKey: "stacktrace",
			LineEnding:    zapcore.DefaultLineEnding,
			// https://godoc.org/go.uber.org/zap/zapcore#EncoderConfig
			// EncodeName is optional but all others must be set
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     timeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   callerEncoder,
		},
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
		DisableStacktrace: true,
	}
}

func isNopLogger(logger *zap.Logger) bool {
	return reflect.DeepEqual(zap.NewNop(), logger)
}

func timeEncoder(time.Time, zapcore.PrimitiveArrayEncoder) {}

func callerEncoder(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
	// zapcore.ShortCallerEncoder(caller, encoder)
}
