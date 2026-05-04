package logger

import (
	"context"

	"go.uber.org/zap"
)

type ServerLogger struct {
	*zap.Logger
}

// NewServerLogger 用底层 zap logger 构造服务日志实例。
func NewServerLogger(logger *zap.Logger) *ServerLogger {
	return &ServerLogger{
		// 这里保留原始 logger，不在构造阶段全局修改 caller skip。
		Logger: logger,
	}
}

// WithContextInfo 记录带上下文的 info 级服务日志。
func (l *ServerLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Info(msg, l.withContext(ctx, fields)...)
}

// WithContextWarn 记录带上下文的 warn 级服务日志。
func (l *ServerLogger) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Warn(msg, l.withContext(ctx, fields)...)
}

// WithContextError 记录带上下文的 error 级服务日志。
func (l *ServerLogger) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Error(msg, l.withContext(ctx, fields)...)
}

// withContext 为服务日志补充 server 类型和 trace 相关字段。
func (l *ServerLogger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return appendContextFields(ctx, "server", fields)
}
