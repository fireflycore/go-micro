# Constant

`constant` 包定义 Firefly 服务间通信中稳定使用的 HTTP Header / gRPC Metadata key。

## 当前保留的核心常量

- `Authorization`：认证令牌，由 authz 在数据面统一消费。
- `XRealIp` / `XForwardedFor`：入口代理透传的客户端 IP 事实。
- `TraceParent` / `TraceState` / `Baggage`：OTEL / W3C Trace Context 传播头。
- `AppVersion`、`UserId` / `AppId` / `TenantId` / `OrgIds` / `RoleIds`：服务内身份上下文。
- `SubjectType` / `InvokeAppId` / `TargetAppId` / `ResourceType` / `ResourcePath` / `DecisionId` / `AuthzContext`：authz 写回的可信上下文。
- `ServiceAppId` / `ServiceInstanceId`：出站调用时注入的服务自身身份。

## 已移除的旧字段

以下字段属于旧网关链路或旧客户端协议，不再作为 go-micro 公共协议继续保留：

- `x-firefly-access-method`
- `x-firefly-route-method`
- `x-firefly-http-gateway-sign`
- `x-firefly-grpc-gateway-sign`
- `x-firefly-gateway-auth-sign`
- `x-firefly-service-auth`

## 说明

`go-micro` 不再把 `x-request-id` 作为业务主 trace 头；链路追踪统一使用 `traceparent` / `tracestate` / `baggage`。
