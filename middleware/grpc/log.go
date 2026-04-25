package gm

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// NewAccessLogger 访问日志中间件
func NewAccessLogger(log *logger.AccessLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if log == nil {
			return handler(ctx, req)
		}

		start := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)
		serviceContext, _ := service.FromContext(ctx)

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

		if v := parseLogMetaKey(md, constant.SourceIp); v != "" {
			fields = append(fields, zap.String("source_ip", v))
		}
		if v := parseLogMetaKey(md, constant.ClientIp); v != "" {
			fields = append(fields, zap.String("client_ip", v))
		}
		if v := parseLogMetaKey(md, constant.InvokeServiceAppId); v != "" {
			fields = append(fields, zap.String("invoke_service_app_id", v))
		}
		if v := parseLogMetaKey(md, constant.InvokeServiceEndpoint); v != "" {
			fields = append(fields, zap.String("invoke_service_endpoint", v))
		}
		if v := parseLogMetaKey(md, constant.TargetServiceAppId); v != "" {
			fields = append(fields, zap.String("target_service_app_id", v))
		}
		if v := parseLogMetaKey(md, constant.TargetServiceEndpoint); v != "" {
			fields = append(fields, zap.String("target_service_endpoint", v))
		}
		if v := parseLogMetaKey(md, constant.SystemName); v != "" {
			fields = append(fields, zap.String("system_name", v))
		}
		if v := parseLogMetaKey(md, constant.ClientName); v != "" {
			fields = append(fields, zap.String("client_name", v))
		}
		if raw := parseLogMetaKey(md, constant.SystemType); raw != "" {
			fields = append(fields, zap.Uint32("system_type", parseInt32OrZero(raw)))
		}
		if raw := parseLogMetaKey(md, constant.ClientType); raw != "" {
			fields = append(fields, zap.Uint32("client_type", parseInt32OrZero(raw)))
		}
		if v := parseLogMetaKey(md, constant.SystemVersion); v != "" {
			fields = append(fields, zap.String("system_version", v))
		}
		if v := parseLogMetaKey(md, constant.ClientVersion); v != "" {
			fields = append(fields, zap.String("client_version", v))
		}
		if v := parseLogMetaKey(md, constant.AppVersion); v != "" {
			fields = append(fields, zap.String("app_version", v))
		}
		if serviceContext != nil {
			if serviceContext.UserId != "" {
				fields = append(fields, zap.String("user_id", serviceContext.UserId))
			}
			if serviceContext.AppId != "" {
				fields = append(fields, zap.String("app_id", serviceContext.AppId))
			}
			if serviceContext.TenantId != "" {
				fields = append(fields, zap.String("tenant_id", serviceContext.TenantId))
			}
			if serviceContext.ServiceAppId != "" {
				fields = append(fields, zap.String("service_app_id", serviceContext.ServiceAppId))
			}
			if serviceContext.ServiceInstanceId != "" {
				fields = append(fields, zap.String("service_instance_id", serviceContext.ServiceInstanceId))
			}
			if serviceContext.RouteMethod != "" {
				fields = append(fields, zap.String("route_method", serviceContext.RouteMethod))
			}
			if serviceContext.AccessMethod != "" {
				fields = append(fields, zap.String("access_method", serviceContext.AccessMethod))
			}
		} else {
			if v := parseLogMetaKey(md, constant.UserId); v != "" {
				fields = append(fields, zap.String("user_id", v))
			}
			if v := parseLogMetaKey(md, constant.AppId); v != "" {
				fields = append(fields, zap.String("app_id", v))
			}
			if v := parseLogMetaKey(md, constant.TenantId); v != "" {
				fields = append(fields, zap.String("tenant_id", v))
			}
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

func parseLogMetaKey(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
