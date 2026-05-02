package logger

import (
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const pkgName = "github.com/fireflycore/go-micro/logger"

func NewZapLogger(config *Config) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	cores := make([]zapcore.Core, 0, 2)
	if config.Console {
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	if config.Remote {
		cores = append(cores, otelzap.NewCore(pkgName))
	}

	if len(cores) == 0 {
		return zap.NewNop()
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
