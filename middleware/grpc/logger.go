package gm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/rpc"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewServiceAccessLogger 服务访问日志中间件，一般在NewInjectServiceContext中间件之前使用
// handle 接收两类日志：b 为结构化 JSON，msg 为人类可读文本行；
// 字段包含 path/request/response/duration/status/trace_id 等，便于统一采集。
func NewServiceAccessLogger(handle func(b []byte, msg string)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)

		// 调用下一个拦截器或服务方法
		resp, err := handler(ctx, req)

		status := 200
		elapsed := time.Since(start)

		if err != nil {
			// 这里用粗粒度状态码做统一聚合（成功/失败），避免与 gRPC status code 绑定过深。
			status = 400
		}

		if handle != nil {
			log := &logger.AccessLogger{}

			// method 字段保持与既有日志采集协议一致（历史字段，值固定）。
			log.Method = constant.RequestMethodGrpc
			log.Path = info.FullMethod

			request, _ := json.Marshal(req)
			log.Request = string(request)

			response, _ := json.Marshal(resp)
			log.Response = string(response)

			log.Duration = uint64(elapsed.Microseconds())
			log.Status = uint32(status)

			// --- Endpoint 解析逻辑变更 ---

			// 1. Trace Identity (由 Gateway 计算并注入)
			log.SourceIp, _ = rpc.ParseMetaKey(md, constant.SourceIp)
			log.ClientIp, _ = rpc.ParseMetaKey(md, constant.ClientIp)

			// 2. Invoke Identity (调用方声明)
			log.InvokeServiceAppId, _ = rpc.ParseMetaKey(md, constant.InvokeServiceAppId)
			log.InvokeServiceEndpoint, _ = rpc.ParseMetaKey(md, constant.InvokeServiceEndpoint)

			// 3. Target Identity (目标路由信息，由 Proxy 注入)
			log.TargetServiceAppId, _ = rpc.ParseMetaKey(md, constant.TargetServiceAppId)
			log.TargetServiceEndpoint, _ = rpc.ParseMetaKey(md, constant.TargetServiceEndpoint)

			// ---------------------------

			log.SystemName, _ = rpc.ParseMetaKey(md, constant.SystemName)
			log.ClientName, _ = rpc.ParseMetaKey(md, constant.ClientName)

			systemType, se := rpc.ParseMetaKey(md, constant.SystemType)
			log.SystemType = parseInt32OrZero(systemType, se)

			clientType, ce := rpc.ParseMetaKey(md, constant.ClientType)
			log.ClientType = parseInt32OrZero(clientType, ce)

			deviceFormFactor, de := rpc.ParseMetaKey(md, constant.DeviceFormFactor)
			log.DeviceFormFactor = parseInt32OrZero(deviceFormFactor, de)

			log.SystemVersion, _ = rpc.ParseMetaKey(md, constant.SystemVersion)
			log.ClientVersion, _ = rpc.ParseMetaKey(md, constant.ClientVersion)
			log.AppVersion, _ = rpc.ParseMetaKey(md, constant.AppVersion)

			traceId, le := rpc.ParseMetaKey(md, constant.TraceId)
			if le != nil {
				// 兼容上游未透传 trace_id 的场景，保证每条日志至少可被唯一关联。
				traceId = uuid.New().String()
			}
			log.TraceId = traceId
			log.UserId, _ = rpc.ParseMetaKey(md, constant.UserId)
			log.AppId, _ = rpc.ParseMetaKey(md, constant.AppId)
			log.TenantId, _ = rpc.ParseMetaKey(md, constant.TenantId)

			b, _ := json.Marshal(log)
			handle(b, fmt.Sprintf("[%s] [GRPC]:[%s] [%s]-[%d] [SourceIp:%s] [ClientIp:%s] [InvokeServiceAppId:%s]\n",
				time.Now().Format(time.DateTime),
				info.FullMethod,
				elapsed.String(),
				status,
				log.SourceIp,
				log.ClientIp,
				log.InvokeServiceAppId,
			))
		}

		return resp, err
	}
}

func parseInt32OrZero(raw string, err error) uint32 {
	if err != nil {
		return 0
	}
	v, pe := strconv.ParseInt(raw, 10, 32)
	if pe != nil {
		return 0
	}
	return uint32(v)
}
