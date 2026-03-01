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
	l.Logger.Info(msg, l.withContext(ctx, fields)...)
}

func (l *Core) WithWarn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *Core) WithError(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Error(msg, l.withContext(ctx, fields)...)
}

func (l *Core) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	if traceId, ok := ctx.Value(constant.TraceId).(string); ok {
		fields = append(fields, zap.String("trace_id", traceId))
	}
	if spanId, ok := ctx.Value(constant.SpanId).(string); ok {
		fields = append(fields, zap.String("parent_id", spanId))
	}
	if userId, ok := ctx.Value(constant.UserId).(string); ok {
		fields = append(fields, zap.String("user_id", userId))
	}
	if appId, ok := ctx.Value(constant.AppId).(string); ok {
		fields = append(fields, zap.String("app_id", appId))
	}
	if tenantId, ok := ctx.Value(constant.TenantId).(string); ok {
		fields = append(fields, zap.String("tenant_id", tenantId))
	}

	return fields
}
