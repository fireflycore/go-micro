# gRPC Middleware (gm)

`gm` 包提供了构建 gRPC 微服务所需的通用中间件（Interceptor）与 OpenTelemetry StatsHandler 适配。

## 功能列表

### 1. Access Logger (`NewAccessLogger`)

提供 gRPC 访问日志记录功能，输出结构化字段（zap fields）。

**特性**：
- **链路关联**：通过 `otelzap` 从 `ctx` 自动关联 trace（要求服务端启用 OTel stats handler，日志使用 `zap.Any("ctx", ctx)`）。
- **身份识别**：从 metadata 读取 `SourceIp/ClientIp/InvokeService*/TargetService*/UserId/AppId/TenantId` 等字段。
- **性能字段**：`duration`（微秒）、`status`（gRPC code）、`path` 等。

**用法**：

```go
import (
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常为 gm
	"google.golang.org/grpc"
)

// 创建 gRPC Server 时注入
s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.NewAccessLogger(log)),
)
```

### 2. Validation 映射 (`ValidationErrorToInvalidArgument`)

将 `protovalidate.ValidationError` 统一转换为 `codes.InvalidArgument`，避免在上层重复判断。

### 3. OpenTelemetry gRPC 埋点（StatsHandler）

`NewOtelServerStatsHandler` 返回 `stats.Handler`，用于 `grpc.StatsHandler(...)` 挂载到服务端，自动完成 trace/metrics 采集与 W3C `traceparent` 传播。

## 组合使用

通常建议使用 `grpc.ChainUnaryInterceptor` 组合多个中间件：

```go
s := grpc.NewServer(
    grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
    grpc.ChainUnaryInterceptor(
        gm.ValidationErrorToInvalidArgument(),
        gm.NewAccessLogger(log),
    ),
)
```
