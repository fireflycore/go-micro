# Constant

`constant` 包只定义 go-micro 多个子包和业务服务共同依赖的公共协议常量。

它不是“所有常量”的集中存放处。错误码、错误原因、authz 服务内部响应头、组件私有白名单和测试辅助值都应留在各自包或服务中。

## 当前保留的核心常量

- `UserAuthority` / `ServiceAuthority`：跨进程传给 authz 校验的 authority header；用户 authority 可透传，服务 authority 每一跳由当前服务覆盖。
- `XRealIp` / `XForwardedFor`：入口代理透传的客户端 IP 事实，用于访问日志和 authz token 状态校验。
- `TraceParent` / `TraceState` / `Baggage`：OTEL / W3C Trace Context 传播头。
- `AppLanguage` / `AppVersion`：客户端应用上下文。
- `Session`、`UserId` / `AppId` / `TenantId` / `OrgIds` / `PostIds` / `RoleIds`：authz 解析用户 authority 后注入的普通身份 metadata key，其中 `AppId` 只表示用户身份中的 app_id。
- `SubjectType` / `InvokeAppId` / `InvokeInstanceId` / `TargetAppId` / `TargetInstanceId` / `ApiMethod` / `ApiPath` / `DecisionId`：authz allow 后写回的普通上下文字段，便于业务日志和排障读取。
- `AuthzSign`：`x-firefly-authz-sign` metadata key，对应 authz 写回的短有效期 compact JWS；可信上下文只从验签后的 JWS payload 读取。
- `RequestMethod*` / `RequestMethod*String`：访问日志和权限资源动作共用的 HTTP/gRPC method 枚举。
- `SubjectTypeAnonymous` / `SubjectTypeUser` / `SubjectTypeService`：authz JWS payload 和服务内上下文共用的主体类型。
- `JWSAlgorithmEdDSA` / `JWSTypeJWT`：服务侧验签 Firefly JWS 时使用的公共 JOSE 字段值。

## 已移除的旧字段

以下字段属于旧网关链路或旧客户端协议，不再作为 go-micro current 协议继续保留：

- `x-firefly-access-method`
- `x-firefly-route-method`
- `x-firefly-http-gateway-sign`
- `x-firefly-grpc-gateway-sign`
- `x-firefly-gateway-auth-sign`
- `x-firefly-service-auth`
- `x-firefly-resource-type`
- `x-firefly-resource-path`
- `authorization` 作为 Firefly current 身份入口
- `x-firefly-authz-error-code`
- `x-firefly-authz-error-reason`
- `x-firefly-token-refresh-required`

## 说明

`go-micro` 不再把 `x-request-id` 作为业务主 trace 头；链路追踪统一使用 `traceparent` / `tracestate` / `baggage`。

`x-firefly-api-method` / `x-firefly-api-path` 是 authz allow 后注入的读取便利字段；业务服务需要可信数据时必须优先验签 `x-firefly-authz-sign`。

`go-micro` 出站调用采用白名单策略，透传用户 authority 和短 TTL `x-firefly-authz-sign`，但不透传普通身份 metadata、标准 `authorization` 头或未知业务 metadata。下一跳 Envoy ext_authz 仍必须按当前 route 重新计算权限，并由 authz 重新签名注入新的结果。

当前业务服务出站白名单由 `authz` 包维护，保留范围为：

- `x-firefly-user-authority`
- `x-firefly-authz-sign`
- `traceparent` / `tracestate` / `baggage`
- `x-real-ip` / `x-forwarded-for`
- `x-firefly-app-language` / `x-firefly-app-version`
- `x-firefly-system-type` / `x-firefly-system-name` / `x-firefly-system-version`
- `x-firefly-client-type` / `x-firefly-client-name` / `x-firefly-client-version`

`x-firefly-service-authority` 不从上游继承，而是在当前这一跳由 go-micro 重新覆盖写入。`x-firefly-service-app-id` / `x-firefly-service-instance-id` 不属于 current 协议，请求链路不再注入。
