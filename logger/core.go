package logger

import (
	"context"

	"go.uber.org/zap"
)

type Core struct {
	logger *zap.Logger
}

func NewLogger(logger *zap.Logger) *Core {
	return &Core{logger}
}

func (l *Core) Info(ctx context.Context, msg string, fields ...zap.Field) {
	l.logger.Info(msg, l.withContext(ctx, fields)...)
}

func (l *Core) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	l.logger.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *Core) Error(ctx context.Context, msg string, fields ...zap.Field) {
	l.logger.Error(msg, l.withContext(ctx, fields)...)
}

func (l *Core) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	return append(fields, zap.Any("ctx", ctx))
}
