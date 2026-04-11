package logger

import (
	"github.com/fireflycore/go-micro/config"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLogger(bootstrapConf config.BootstrapConfig) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	cores := make([]zapcore.Core, 0, 2)
	if bootstrapConf.IsEnableConsole() {
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	if bootstrapConf.IsEnableRemote() {
		cores = append(cores, otelzap.NewCore(bootstrapConf.GetAppName()))
	}

	if len(cores) == 0 {
		return zap.NewNop()
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
