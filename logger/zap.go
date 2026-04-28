package logger

import (
	"github.com/fireflycore/go-micro/app"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLogger(appConfig *app.Config, loggerConfig *Config) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	cores := make([]zapcore.Core, 0, 2)
	if loggerConfig.EnableConsole {
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	if loggerConfig.EnableConsole {
		cores = append(cores, otelzap.NewCore(appConfig.Name))
	}

	if len(cores) == 0 {
		return zap.NewNop()
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
