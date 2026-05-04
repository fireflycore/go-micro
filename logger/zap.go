package logger

import (
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// pkgName 作为 otel instrumentation scope name 使用。
const pkgName = "github.com/fireflycore/go-micro/logger"

// NewZapLogger 根据配置构造统一的 zap logger。
func NewZapLogger(config *Config) *zap.Logger {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)

	// 最多同时挂两个 core：一个本地 console，一个远端 otel。
	cores := make([]zapcore.Core, 0, 2)
	// 开启 console 时，给普通输出 core 套一层 ctx 过滤包装。
	if config.Console {
		cores = append(cores, NewContextOmittingCore(NewConsoleCore(atomicLevel)))
	}
	// 开启 remote 时，直接挂上 otelzap core。
	if config.Remote {
		cores = append(cores, otelzap.NewCore(pkgName))
	}

	// 一个输出目标都没启用时，返回 nop logger 避免上层判空。
	if len(cores) == 0 {
		return zap.NewNop()
	}

	// 把多个 core 合并成一个 logger，并保留调用方位置信息。
	return zap.New(zapcore.NewTee(cores...), zap.AddCaller())
}
