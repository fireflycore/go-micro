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
		// 服务日志多包了一层方法调用，这里跳过一层 caller，让日志位置回到业务调用点。
		Logger: logger.WithOptions(zap.AddCallerSkip(1)),
	}
}

// WithContextInfo 记录带上下文的 info 级服务日志。
func (l *ServerLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	l.Info(msg, l.withContext(ctx, fields)...)
}

// WithContextWarn 记录带上下文的 warn 级服务日志。
func (l *ServerLogger) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Warn(msg, l.withContext(ctx, fields)...)
}

// WithContextError 记录带上下文的 error 级服务日志。
func (l *ServerLogger) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Error(msg, l.withContext(ctx, fields)...)
}

// withContext 为服务日志补充 server 类型和 trace 相关字段。
func (l *ServerLogger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return appendContextFields(ctx, "server", fields)
}
