package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/rpc"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GrpcAccessLogger GRPC访问日志中间件，一般在WithServiceContext之前使用
// handle 接收两类日志：b 为结构化 JSON，msg 为人类可读文本行；
// 字段包含 path/request/response/duration/status/trace_id 等，便于统一采集。
func GrpcAccessLogger(handle func(b []byte, msg string)) grpc.UnaryServerInterceptor {
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
			loggerMap := make(map[string]interface{})

			// method 字段保持与既有日志采集协议一致（历史字段，值固定）。
			loggerMap["method"] = 5
			loggerMap["path"] = info.FullMethod

			request, _ := json.Marshal(req)
			loggerMap["request"] = string(request)

			response, _ := json.Marshal(resp)
			loggerMap["response"] = string(response)

			loggerMap["duration"] = elapsed.Microseconds()
			loggerMap["status"] = status
			if err != nil {
				loggerMap["error"] = err.Error()
			}

			// --- Endpoint 解析逻辑变更 ---

			// 1. Trace Identity (由 Gateway 计算并注入)
			sourceIp, _ := rpc.ParseMetaKey(md, constant.SourceIp)
			clientIp, _ := rpc.ParseMetaKey(md, constant.ClientIp)

			// 兼容旧逻辑或降级：如果 ClientEndpoint 为空，尝试取 Peer IP (虽然这通常在网关层做，但 Service 层兜底也没坏处)
			// 但基于新规范，Service 层应信任网关传递的 Header。

			loggerMap["source_ip"] = sourceIp
			loggerMap["client_ip"] = clientIp

			// 2. Invoke Identity (调用方声明)
			invokeServiceAppId, _ := rpc.ParseMetaKey(md, constant.InvokeServiceAppId)
			invokeServiceEndpoint, _ := rpc.ParseMetaKey(md, constant.InvokeServiceEndpoint)
			loggerMap["invoke_service_app_id"] = invokeServiceAppId
			loggerMap["invoke_service_endpoint"] = invokeServiceEndpoint

			// 3. Target Identity (目标路由信息，由 Proxy 注入)
			targetServiceAppId, _ := rpc.ParseMetaKey(md, constant.TargetServiceAppId)
			targetServiceEndpoint, _ := rpc.ParseMetaKey(md, constant.TargetServiceEndpoint)
			loggerMap["target_service_app_id"] = targetServiceAppId
			loggerMap["target_service_endpoint"] = targetServiceEndpoint

			// ---------------------------

			loggerMap["system_name"], _ = rpc.ParseMetaKey(md, constant.SystemName)
			loggerMap["client_name"], _ = rpc.ParseMetaKey(md, constant.ClientName)

			systemType, se := rpc.ParseMetaKey(md, constant.SystemType)
			loggerMap["system_type"] = parseInt32OrZero(systemType, se)
			clientType, ce := rpc.ParseMetaKey(md, constant.ClientType)
			loggerMap["client_type"] = parseInt32OrZero(clientType, ce)
			deviceFormFactor, de := rpc.ParseMetaKey(md, constant.DeviceFormFactor)
			loggerMap["device_form_factor"] = parseInt32OrZero(deviceFormFactor, de)

			loggerMap["system_version"], _ = rpc.ParseMetaKey(md, constant.SystemVersion)
			loggerMap["client_version"], _ = rpc.ParseMetaKey(md, constant.ClientVersion)
			loggerMap["app_version"], _ = rpc.ParseMetaKey(md, constant.AppVersion)

			traceId, le := rpc.ParseMetaKey(md, constant.TraceId)
			if le != nil {
				// 兼容上游未透传 trace_id 的场景，保证每条日志至少可被唯一关联。
				traceId = uuid.New().String()
			}
			loggerMap["trace_id"] = traceId
			loggerMap["user_id"], _ = rpc.ParseMetaKey(md, constant.UserId)
			loggerMap["app_id"], _ = rpc.ParseMetaKey(md, constant.AppId)

			b, _ := json.Marshal(loggerMap)
			handle(b, fmt.Sprintf("[%s] [GRPC]:[%s] [%s]-[%d] [SourceIp:%s] [ClientIp:%s] [InvokeServiceAppId:%s]\n",
				time.Now().Format(time.DateTime),
				info.FullMethod,
				elapsed.String(),
				status,
				sourceIp,
				clientIp,
				invokeServiceAppId,
			))
		}

		return resp, err
	}
}

func parseInt32OrZero(raw string, err error) int64 {
	if err != nil {
		return 0
	}
	v, pe := strconv.ParseInt(raw, 10, 32)
	if pe != nil {
		return 0
	}
	return v
}
