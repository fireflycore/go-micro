package logger

import (
	"context"

	"go.uber.org/zap"
)

type Core struct {
	*zap.Logger
}

func NewLogger(logger *zap.Logger) *Core {
	return &Core{logger}
}

func (l *Core) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
	l.Info(msg, l.withContext(ctx, fields)...)
}

func (l *Core) WithContextWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *Core) WithContextError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Error(msg, l.withContext(ctx, fields)...)
}

func (l *Core) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return append(fields, zap.String("log_type", "server"), zap.Any("ctx", ctx))
}
