package gm

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/rpc"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// NewAccessLogger 访问日志中间件
func NewAccessLogger(log *logger.AccessLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 注入 TraceId 到 Response Header (Trailer)
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			_ = grpc.SetHeader(ctx, metadata.Pairs(constant.TraceId, span.SpanContext().TraceID().String()))
		}

		if log == nil {
			return handler(ctx, req)
		}

		start := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)

		// 调用下一个拦截器或服务方法
		resp, err := handler(ctx, req)

		elapsed := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		fields := make([]zap.Field, 0, 32)
		fields = append(fields,
			zap.String("log_type", "access"),
			zap.String("protocol", "grpc"),
			zap.String("method", constant.RequestMethodGrpcString),
			zap.String("path", info.FullMethod),
			zap.Uint64("duration", uint64(elapsed.Microseconds())),
			zap.Uint32("status", uint32(code)),
		)

		if request, e := json.Marshal(req); e == nil {
			fields = append(fields, zap.ByteString("request", request))
		}
		if response, e := json.Marshal(resp); e == nil {
			fields = append(fields, zap.ByteString("response", response))
		}

		if v, e := rpc.ParseMetaKey(md, constant.SourceIp); e == nil && v != "" {
			fields = append(fields, zap.String("source_ip", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.ClientIp); e == nil && v != "" {
			fields = append(fields, zap.String("client_ip", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.InvokeServiceAppId); e == nil && v != "" {
			fields = append(fields, zap.String("invoke_service_app_id", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.InvokeServiceEndpoint); e == nil && v != "" {
			fields = append(fields, zap.String("invoke_service_endpoint", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.TargetServiceAppId); e == nil && v != "" {
			fields = append(fields, zap.String("target_service_app_id", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.TargetServiceEndpoint); e == nil && v != "" {
			fields = append(fields, zap.String("target_service_endpoint", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.SystemName); e == nil && v != "" {
			fields = append(fields, zap.String("system_name", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.ClientName); e == nil && v != "" {
			fields = append(fields, zap.String("client_name", v))
		}
		if raw, e := rpc.ParseMetaKey(md, constant.SystemType); e == nil {
			fields = append(fields, zap.Uint32("system_type", parseInt32OrZero(raw)))
		}
		if raw, e := rpc.ParseMetaKey(md, constant.ClientType); e == nil {
			fields = append(fields, zap.Uint32("client_type", parseInt32OrZero(raw)))
		}
		if v, e := rpc.ParseMetaKey(md, constant.SystemVersion); e == nil && v != "" {
			fields = append(fields, zap.String("system_version", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.ClientVersion); e == nil && v != "" {
			fields = append(fields, zap.String("client_version", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.AppVersion); e == nil && v != "" {
			fields = append(fields, zap.String("app_version", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.UserId); e == nil && v != "" {
			fields = append(fields, zap.String("user_id", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.AppId); e == nil && v != "" {
			fields = append(fields, zap.String("app_id", v))
		}
		if v, e := rpc.ParseMetaKey(md, constant.TenantId); e == nil && v != "" {
			fields = append(fields, zap.String("tenant_id", v))
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			log.WithContextError(ctx, constant.GrpcAccessLog, fields...)
		} else {
			log.WithContextInfo(ctx, constant.GrpcAccessLog, fields...)
		}

		return resp, err
	}
}

func parseInt32OrZero(raw string) uint32 {
	v, pe := strconv.ParseInt(raw, 10, 32)
	if pe != nil {
		return 0
	}
	return uint32(v)
}
