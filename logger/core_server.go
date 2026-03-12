package logger

import (
	"context"

	"go.uber.org/zap"
)

type ServerLogger struct {
	*zap.Logger
}

func NewServerLogger(logger *zap.Logger) *ServerLogger {
	return &ServerLogger{
		Logger: logger,
	}
}

func (l *ServerLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	l.Info(msg, l.withContext(ctx, fields)...)
}

func (l *ServerLogger) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *ServerLogger) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Error(msg, l.withContext(ctx, fields)...)
}

func (l *ServerLogger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return append(fields, zap.String("log_type", "server"), zap.Any("ctx", ctx))
}
