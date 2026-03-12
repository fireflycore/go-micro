package logger

import (
	"context"

	"go.uber.org/zap"
)

type AccessLogger struct {
	*zap.Logger
}

func NewAccessLogger(logger *zap.Logger) *AccessLogger {
	return &AccessLogger{
		Logger: logger,
	}
}

func (l *AccessLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	l.Info(msg, l.withContext(ctx, fields)...)
}

func (l *AccessLogger) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *AccessLogger) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Error(msg, l.withContext(ctx, fields)...)
}

func (l *AccessLogger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return append(fields, zap.String("log_type", "access"), zap.Any("ctx", ctx))
}
