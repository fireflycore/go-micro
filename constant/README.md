# Constant

`constant` 包定义了微服务通信中通用的 HTTP Header / gRPC Metadata Key。

## 常用常量

这些常量主要用于 gRPC Metadata 或 HTTP Header 中传递上下文信息。

### 标准头

- `Authorization`：认证令牌（`authorization`），由 authz 在数据面统一消费。
- `XRealIp`：入口代理透传真实 IP（`x-real-ip`）。
- `XForwardedFor`：代理链路 IP 列表（`x-forwarded-for`）。
- `TraceParent` / `TraceState` / `Baggage`：W3C Trace Context 与 Baggage 标准头。

链路追踪统一使用 OpenTelemetry 自动注入/提取的 `traceparent` / `tracestate`，不再定义 `x-firefly-trace-id` 或 `x-request-id` 作为链路主键。

### 身份上下文

- `Session`：会话标识（`x-firefly-session`）。
- `UserId`：用户 ID（`x-firefly-user-id`）。
- `AppId`：调用方应用 ID（`x-firefly-app-id`），authz allow 后与 `InvokeAppId` 保持一致。
- `TenantId`：租户 ID（`x-firefly-tenant-id`）。
- `OrgIds` / `RoleIds`：组织与角色 ID 列表。

### Authz 可信上下文

- `SubjectType`：主体类型（`anonymous` / `user` / `service`）。
- `InvokeAppId`：调用方应用 ID。
- `TargetAppId`：被访问资源所属应用 ID。
- `ResourceType`：权限动作，HTTP 为方法名，gRPC 为 `GRPC`。
- `ResourcePath`：权限资源路径。
- `DecisionId`：authz allow 决策 ID。
- `AuthzContext`：authz 写入的短有效期签名 JWS，是服务侧信任根。

### 客户端信息

- `SystemName` / `SystemVersion`：系统名称与版本。
- `ClientName` / `ClientVersion`：客户端名称与版本。
- `SystemType` / `ClientType`：系统/客户端类型。
- `AppVersion` / `AppLanguage`：应用版本 / 语言。

### 服务身份

- `ServiceAppId` / `ServiceInstanceId`：当前服务发起下游 gRPC 调用时由 `go-micro/invocation` 注入的服务身份。

### 已移除字段

以下字段属于旧 Http-Gateway / Grpc-Gateway 链路，不再保留为 `go-micro` 公共协议：

- `x-firefly-access-method`
- `x-firefly-route-method`
- `x-firefly-http-gateway-sign`
- `x-firefly-grpc-gateway-sign`
- `x-firefly-gateway-auth-sign`
- `x-firefly-service-auth`

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
