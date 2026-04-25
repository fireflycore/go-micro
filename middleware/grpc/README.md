# gRPC Middleware (gm)

`gm` 包提供了构建 gRPC 微服务所需的通用中间件（Interceptor）与 OpenTelemetry StatsHandler 适配。

## 功能列表

### 1. ServiceContext (`NewServiceContextUnaryInterceptor`)

在请求入口统一完成：

- 解析入站 metadata 中的用户与路由字段
- 构造服务内唯一主上下文 `service.Context`
- 从当前 OTel span 提取 trace 标识快照
- 补齐当前服务自身身份（`ServiceAppId`、`ServiceInstanceId`）

**推荐用途**：
- 在 gRPC 服务端入口统一注入 `service.Context`
- access log 与审计日志统一读取字段
- 在请求入口统一完成服务内主上下文注入

`gm` 当前只负责服务端入站中间件语义，不再定义服务内主上下文模型；业务代码应从 `go-micro/service` 读取 `service.Context`，出站调用统一由 `go-micro/invocation` 直接基于当前 gRPC context 与 OTel trace 处理。

### 2. Access Logger (`NewAccessLogger`)

提供 gRPC 访问日志记录功能，输出结构化字段（zap fields）。

**特性**：
- **链路关联**：通过 `otelzap` 从 `ctx` 自动关联 trace（要求服务端启用 OTel stats handler，日志使用 `zap.Any("ctx", ctx)`）。
- **身份识别**：优先读取 `ServiceContext`，必要时再回退到 metadata 中的兼容字段。
- **性能字段**：`duration`（微秒）、`status`（gRPC code）、`path` 等。

**用法**：

```go
import (
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常为 gm
	"google.golang.org/grpc"
)

// 创建 gRPC Server 时注入
accessLog := logger.NewAccessLogger(zl)
s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.NewAccessLogger(accessLog)),
)
```

### 3. Validation 映射 (`ValidationErrorToInvalidArgument`)

将 `protovalidate.ValidationError` 统一转换为 `codes.InvalidArgument`，避免在上层重复判断。

### 4. OpenTelemetry gRPC 埋点（StatsHandler）

`NewOtelServerStatsHandler` 返回 `stats.Handler`，用于 `grpc.StatsHandler(...)` 挂载到服务端，自动完成 trace/metrics 采集与 W3C `traceparent` 传播。

## 组合使用

通常建议使用 `grpc.ChainUnaryInterceptor` 组合多个中间件：

```go
s := grpc.NewServer(
    grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
    grpc.ChainUnaryInterceptor(
        gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
            ServiceAppId:      "auth",
            ServiceInstanceId: "auth-1",
        }),
        gm.ValidationErrorToInvalidArgument(),
        gm.NewAccessLogger(accessLog),
    ),
)
```
