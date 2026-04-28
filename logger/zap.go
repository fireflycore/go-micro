package logger

import (
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLogger(appName string, config *Config) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	cores := make([]zapcore.Core, 0, 2)
	if config.Console {
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	if config.Console {
		cores = append(cores, otelzap.NewCore(appName))
	}

	if len(cores) == 0 {
		return zap.NewNop()
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
