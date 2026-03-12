package logger

import (
	"github.com/fireflycore/go-micro/conf"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLogger(bootstrapConf conf.BootstrapConf, opts ...otelzap.Option) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	cores := make([]zapcore.Core, 0, 2)
	if bootstrapConf.GetLoggerConsole() {
		cores = append(cores, NewConsoleCore(atomicLevel))
	}
	if bootstrapConf.GetLoggerRemote() {
		cores = append(cores, otelzap.NewCore(bootstrapConf.GetAppName(), opts...))
	}

	if len(cores) == 0 {
		return zap.NewNop()
	}

	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
