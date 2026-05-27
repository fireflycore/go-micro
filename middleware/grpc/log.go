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

const grpcHealthCheckFullMethod = "/grpc.health.v1.Health/Check"

// AccessLoggerOptions 定义 gRPC 访问日志中间件的可选配置。
type AccessLoggerOptions struct {
	// SkipMethods 表示需要跳过访问日志记录的完整 gRPC 方法名列表。
	// 例如：/grpc.health.v1.Health/Check
	SkipMethods []string
}

// NewAccessLogger 访问日志中间件
//
// 设计说明：
// - 默认跳过 gRPC health check，避免探针请求刷屏访问日志。
// - 业务方也可以通过 options 追加自定义的跳过方法列表。
func NewAccessLogger(log *logger.AccessLogger, options ...AccessLoggerOptions) grpc.UnaryServerInterceptor {
	// 预先整理跳过规则，避免每次请求都重复构造。
	skipMethods := buildAccessLogSkipMethods(options...)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 没有 logger 时直接透传请求。
		if log == nil {
			return handler(ctx, req)
		}
		// 命中跳过规则时，直接放行，不记录访问日志。
		if shouldSkipAccessLog(info, skipMethods) {
			return handler(ctx, req)
		}

		// 进入日志中间件时记录开始时间，用于后面计算耗时。
		start := time.Now()
		// 提前提取 metadata，后续用于补充访问日志字段。
		md, _ := metadata.FromIncomingContext(ctx)
		// 读取服务内部统一的 ServiceContext，优先复用已结构化的上下文数据。
		serviceContext, _ := service.FromContext(ctx)

		// 调用下一个拦截器或服务方法
		resp, err := handler(ctx, req)

		// 计算请求实际耗时。
		elapsed := time.Since(start)

		// 默认按成功状态处理，若有错误再覆盖成对应 grpc code。
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		// 预分配字段切片，减少 append 过程中的扩容。
		fields := make([]zap.Field, 0, 32)
		// 补齐访问日志的基础字段。
		fields = append(fields,
			zap.String("log_type", "access"),
			zap.String("protocol", "grpc"),
			zap.String("method", constant.RequestMethodGrpcString),
			zap.String("path", info.FullMethod),
			zap.Uint64("duration", uint64(elapsed.Microseconds())),
			zap.Uint32("status", uint32(code)),
		)

		// 请求体可序列化时，记录请求报文。
		if request, e := json.Marshal(req); e == nil {
			fields = append(fields, zap.ByteString("request", request))
		}
		// 响应体可序列化时，记录响应报文。
		if response, e := json.Marshal(resp); e == nil {
			fields = append(fields, zap.ByteString("response", response))
		}

		// 从入站 metadata 中提取客户端 IP。
		if v := parseLogMetaKey(md, constant.XRealIp); v != "" {
			fields = append(fields, zap.String("client_ip", v))
		}

		// 优先记录调用方服务身份信息。
		if v := parseLogMetaKey(md, constant.ServiceAppId); v != "" {
			fields = append(fields, zap.String("service_app_id", v))
		}
		if v := parseLogMetaKey(md, constant.ServiceInstanceId); v != "" {
			fields = append(fields, zap.String("service_instance_id", v))
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
		// 若上游已在入口构建 ServiceContext，则优先使用结构化后的字段。
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
			if serviceContext.SubjectType != "" {
				fields = append(fields, zap.String("subject_type", serviceContext.SubjectType))
			}
			if serviceContext.InvokeAppId != "" {
				fields = append(fields, zap.String("invoke_app_id", serviceContext.InvokeAppId))
			}
			if serviceContext.TargetAppId != "" {
				fields = append(fields, zap.String("target_app_id", serviceContext.TargetAppId))
			}
			if serviceContext.ResourceType != "" {
				fields = append(fields, zap.String("resource_type", serviceContext.ResourceType))
			}
			if serviceContext.ResourcePath != "" {
				fields = append(fields, zap.String("resource_path", serviceContext.ResourcePath))
			}
			if serviceContext.DecisionId != "" {
				fields = append(fields, zap.String("decision_id", serviceContext.DecisionId))
			}
			if serviceContext.ServiceAppId != "" {
				fields = append(fields, zap.String("service_app_id", serviceContext.ServiceAppId))
			}
			if serviceContext.ServiceInstanceId != "" {
				fields = append(fields, zap.String("service_instance_id", serviceContext.ServiceInstanceId))
			}
		} else {
			// 没有 ServiceContext 时，再回退到原始 metadata 中兜底提取。
			if v := parseLogMetaKey(md, constant.UserId); v != "" {
				fields = append(fields, zap.String("user_id", v))
			}
			if v := parseLogMetaKey(md, constant.AppId); v != "" {
				fields = append(fields, zap.String("app_id", v))
			}
			if v := parseLogMetaKey(md, constant.TenantId); v != "" {
				fields = append(fields, zap.String("tenant_id", v))
			}
			if v := parseLogMetaKey(md, constant.SubjectType); v != "" {
				fields = append(fields, zap.String("subject_type", v))
			}
			if v := parseLogMetaKey(md, constant.InvokeAppId); v != "" {
				fields = append(fields, zap.String("invoke_app_id", v))
			}
			if v := parseLogMetaKey(md, constant.TargetAppId); v != "" {
				fields = append(fields, zap.String("target_app_id", v))
			}
			if v := parseLogMetaKey(md, constant.ResourceType); v != "" {
				fields = append(fields, zap.String("resource_type", v))
			}
			if v := parseLogMetaKey(md, constant.ResourcePath); v != "" {
				fields = append(fields, zap.String("resource_path", v))
			}
			if v := parseLogMetaKey(md, constant.DecisionId); v != "" {
				fields = append(fields, zap.String("decision_id", v))
			}
		}

		// 有错误时按 error 级别记录，并附带 error 字段。
		if err != nil {
			fields = append(fields, zap.Error(err))
			log.WithContextError(ctx, constant.GrpcAccessLog, fields...)
		} else {
			// 成功请求按 info 级别记录。
			log.WithContextInfo(ctx, constant.GrpcAccessLog, fields...)
		}

		// 返回下游处理结果。
		return resp, err
	}
}

// buildAccessLogSkipMethods 构造最终的跳过方法集合。
func buildAccessLogSkipMethods(options ...AccessLoggerOptions) map[string]struct{} {
	// 默认跳过健康检查，避免探针请求占满访问日志。
	methods := map[string]struct{}{
		grpcHealthCheckFullMethod: {},
	}

	// 合并业务方传入的自定义跳过列表。
	for _, option := range options {
		for _, method := range option.SkipMethods {
			if method == "" {
				continue
			}
			methods[method] = struct{}{}
		}
	}

	return methods
}

// shouldSkipAccessLog 判断当前请求是否应跳过访问日志。
func shouldSkipAccessLog(info *grpc.UnaryServerInfo, skipMethods map[string]struct{}) bool {
	// 缺少方法信息时不跳过，保持默认记录行为。
	if info == nil || info.FullMethod == "" {
		return false
	}
	// 命中跳过集合则直接返回 true。
	_, ok := skipMethods[info.FullMethod]
	return ok
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
