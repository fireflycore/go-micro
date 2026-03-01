package logger

import (
	"context"

	"github.com/fireflycore/go-micro/constant"
	"go.uber.org/zap"
)

// LogLevel 定义日志级别枚举。
type LogLevel uint32

const (
	Info  LogLevel = 1 // Info 普通级别。
	Warn  LogLevel = 2 // Warn 警告级别。
	Error LogLevel = 3 // Error 错误级别。
)

type Core struct {
	*zap.Logger
}

func NewLogger(logger *zap.Logger) *Core {
	return &Core{logger}
}

func (l *Core) WithInfo(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Info(msg, l.withCtx(ctx, fields)...)
}

func (l *Core) WithWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, l.withCtx(ctx, fields)...)
}

func (l *Core) WithError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Error(msg, l.withCtx(ctx, fields)...)
}

func (l *Core) withCtx(ctx context.Context, fields []zap.Field) []zap.Field {
	traceId, _ := ctx.Value(constant.TraceId).(string)
	spanId, _ := ctx.Value(constant.SpanId).(string)

	return append(fields, zap.String("trace_id", traceId), zap.String("parent_id", spanId))
}
