package logger

import (
	"context"
	"github.com/fireflycore/go-micro/rpc"
	"google.golang.org/grpc/metadata"

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
	md, _ := metadata.FromIncomingContext(ctx)

	if traceId, err := rpc.ParseMetaKey(md, constant.TraceId); err == nil {
		fields = append(fields, zap.String("trace_id", traceId))
	}
	if spanId, err := rpc.ParseMetaKey(md, constant.SpanId); err == nil {
		fields = append(fields, zap.String("parent_id", spanId))
	}
	if userId, err := rpc.ParseMetaKey(md, constant.UserId); err == nil {
		fields = append(fields, zap.String("user_id", userId))
	}
	if appId, err := rpc.ParseMetaKey(md, constant.AppId); err == nil {
		fields = append(fields, zap.String("app_id", appId))
	}
	if tenantId, err := rpc.ParseMetaKey(md, constant.TenantId); err == nil {
		fields = append(fields, zap.String("tenant_id", tenantId))
	}

	return fields
}
