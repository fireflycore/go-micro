package gm

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/werror"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const grpcHealthCheckFullMethod = "/grpc.health.v1.Health/Check"

// AccessLoggerOptions 配置无需输出访问日志的精确方法集合。
type AccessLoggerOptions struct {
	// SkipMethods 是无需记录访问日志的完整方法，不支持前缀或通配符。
	SkipMethods []string
}

// NewAccessLogger 只读取结构化可信上下文，不记录原始请求体、凭证或异常消息。
// log 必须非 nil；关闭访问日志应由装配层明确不安装中间件。
func NewAccessLogger(log *logger.AccessLogger, options ...AccessLoggerOptions) grpc.UnaryServerInterceptor {
	skip := map[string]bool{grpcHealthCheckFullMethod: true}
	for _, option := range options {
		for _, method := range option.SkipMethods {
			skip[method] = true
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}
		start := time.Now()
		resp, err := handler(ctx, req)

		fields := []zap.Field{
			zap.String("log_type", "access"),
			zap.String("protocol", "grpc"),
			zap.String("method", constant.RequestMethodGrpcString),
			zap.String("path", info.FullMethod),
			zap.Uint64("duration", uint64(time.Since(start).Microseconds())),
			zap.Uint32("status", uint32(status.Code(err))),
		}
		fields = append(fields, logger.ContextFields(ctx)...)

		if identity, ok := werror.ErrorInfoOf(err); ok {
			fields = append(fields, zap.String("error_domain", identity.GetDomain()), zap.String("error_reason", identity.GetReason()))
		}

		// 异常消息、请求体和任意 metadata 可能包含凭证，只记录安全错误标识。
		if err != nil {
			log.WithContextError(ctx, constant.GrpcAccessLog, fields...)
		} else {
			log.WithContextInfo(ctx, constant.GrpcAccessLog, fields...)
		}

		return resp, err
	}
}
