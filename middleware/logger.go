package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	micro "github.com/fireflycore/go-micro/core"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GrpcAccessLogger 返回一个 gRPC Unary 拦截器，用于输出访问日志。
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

			loggerMap["ip"], _ = micro.ParseMetaKey(md, "client-ip")

			loggerMap["system_name"], _ = micro.ParseMetaKey(md, "system-name")
			loggerMap["client_name"], _ = micro.ParseMetaKey(md, "client-name")

			systemType, se := micro.ParseMetaKey(md, "system-type")
			loggerMap["system_type"] = parseInt32OrZero(systemType, se)
			clientType, ce := micro.ParseMetaKey(md, "client-type")
			loggerMap["client_type"] = parseInt32OrZero(clientType, ce)
			deviceFormFactor, de := micro.ParseMetaKey(md, "device-form-factor")
			loggerMap["device_form_factor"] = parseInt32OrZero(deviceFormFactor, de)

			loggerMap["system_version"], _ = micro.ParseMetaKey(md, "system-version")
			loggerMap["client_version"], _ = micro.ParseMetaKey(md, "client-version")
			loggerMap["app_version"], _ = micro.ParseMetaKey(md, "app-version")

			traceId, le := micro.ParseMetaKey(md, "trace-id")
			if le != nil {
				// 兼容上游未透传 trace_id 的场景，保证每条日志至少可被唯一关联。
				traceId = uuid.New().String()
			}
			loggerMap["trace_id"] = traceId
			loggerMap["user_id"], _ = micro.ParseMetaKey(md, "user-id")
			loggerMap["app_id"], _ = micro.ParseMetaKey(md, "app-id")

			b, _ := json.Marshal(loggerMap)
			handle(b, fmt.Sprintf("[%s] [GRPC]:[%s] [%s]-[%d]\n", time.Now().Format(time.DateTime), info.FullMethod, elapsed.String(), status))
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
