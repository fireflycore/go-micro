# Constant

`constant` 包定义了微服务通信中通用的 HTTP Header / gRPC Metadata Key。

## 常用常量

这些常量主要用于 gRPC Metadata 或 HTTP Header 中传递上下文信息。

### 身份与追踪
- `Session`：会话标识（`x-firefly-session`）
- `Authorization`：认证令牌（`authorization`）
- `UserId`：用户 ID（`x-firefly-user-id`）
- `AppId`：应用 ID（`x-firefly-app-id`）
- `TenantId`：租户 ID（`x-firefly-tenant-id`）
- `ClientIp`：客户端 IP（`x-firefly-client-ip`）
- `XRealIp`：反向代理透传真实 IP（`x-real-ip`）

> 链路追踪统一使用 OpenTelemetry tracer 注入/提取的 W3C `traceparent`/`tracestate`。
> 历史字段 `x-trace-id` 已从常量定义中移除，不应再作为服务间 header 或 metadata 使用。

### 客户端信息
- `SystemName` / `SystemVersion`：系统名称与版本
- `ClientName` / `ClientVersion`：客户端名称与版本
- `SystemType` / `ClientType`：系统/客户端类型
- `DeviceFormFactor`：设备形态
- `AppVersion` / `AppLanguage`：应用版本 / 语言

### 使用示例

```go
import "google.golang.org/grpc/metadata"
import "github.com/fireflycore/go-micro/constant"

// 从 gRPC metadata 获取用户信息字段
md, _ := metadata.FromIncomingContext(ctx)
values := md.Get(constant.UserId)
if len(values) > 0 {
	userId := values[0]
	_ = userId
}
```

## 规范设计

### Header Key 命名规范
- 所有自定义 Header Key 必须使用小写字母。
- 单词之间使用短横线 `-` 分隔（kebab-case）。
- 必须使用统一的前缀 `x-firefly-`，以区别于标准 HTTP Header 或其他框架 Header。

### 为什么使用静态常量前缀？

我们选择使用硬编码的静态常量前缀（`x-firefly-`），而不采用运行时动态配置前缀（如 `x-{brand}-`），基于以下考量：

1.  **协议标准化（Protocol Standardization）**：
    Header Key 属于通信协议的一部分。如同 HTTP 标准头（`Content-Type`）或 OTel 标准头（`traceparent`）一样，协议的稳定性至关重要。统一的前缀保证了 Firefly 生态内所有组件（网关、SDK、中间件、探针）的无缝互通，无需复杂的协商配置。

2.  **生态兼容性（Ecosystem Compatibility）**：
    如果允许不同服务或不同部署环境使用不同的 Header 前缀，会导致跨服务调用、跨品牌合作、以及第三方工具集成变得异常复杂。维护多套协议标准会导致生态割裂。

3.  **性能与开发体验（Performance & DX）**：
    静态常量在编译期确定，无运行时拼接开销，且对 IDE 友好（可跳转、可补全）。相比之下，动态前缀需要运行时计算或全局变量注入，增加了初始化复杂度和并发风险。

4.  **业务隔离（Business Isolation）**：
    品牌或租户的隔离应当通过 Header **Value**（如 `x-firefly-tenant-id`）来实现，而不是通过修改 Header **Key**。
