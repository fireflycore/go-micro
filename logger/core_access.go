package logger

import (
	"context"

	"go.uber.org/zap"
)

// AccessLogger 是访问日志的轻量封装。
type AccessLogger struct {
	*zap.Logger
}

// NewAccessLogger 用底层 zap logger 构造访问日志实例。
func NewAccessLogger(logger *zap.Logger) *AccessLogger {
	return &AccessLogger{
		// 这里保留原始 logger，不在构造阶段全局修改 caller skip。
		Logger: logger,
	}
}

// WithContextInfo 记录带上下文的 info 级访问日志。
func (l *AccessLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Info(msg, l.withContext(ctx, fields)...)
}

// WithContextWarn 记录带上下文的 warn 级访问日志。
func (l *AccessLogger) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Warn(msg, l.withContext(ctx, fields)...)
}

// WithContextError 记录带上下文的 error 级访问日志。
func (l *AccessLogger) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	// 只有包装方法本身需要额外跳过一层 caller，避免定位到当前文件。
	l.WithOptions(zap.AddCallerSkip(1)).Error(msg, l.withContext(ctx, fields)...)
}

// withContext 为访问日志补充 access 类型和 trace 相关字段。
func (l *AccessLogger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return appendContextFields(ctx, "access", fields)
}
