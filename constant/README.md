# Constant

`constant` 包定义 go-micro 多个子包和业务服务共同依赖的公共协议常量，不集中存放业务错误码、组件私有白名单或测试辅助值。

## 当前核心常量

- `UserAuthority` / `ServiceAuthority`：跨进程传给 authz 校验的凭证头。用户凭证通过框架私有路径继承，服务凭证每一跳由当前服务 provider 提供；provider 缺失或失败时拒绝调用。
- `AuthzSign`：本跳入口的短时 compact JWS 验签输入，每跳重新签发，固定 60 秒；原文不进入公开业务上下文，不向下一跳传播。
- `Session` / `UserId` / `AppId` / `TenantId` / `SubjectType` / `DecisionId`：六项明文 authz 观测头，不能据此建立身份或进行权限判断。用户身份只从验签结果建立，session 不默认写入日志。
- `InvokeMethod` / `InvokePath`：上一跳服务入口观测头；出站以当前实际 `ServiceMethod / ServicePath` 覆盖，不作为授权或投递绑定。
- `AppLanguage` / `AppVersion`、`SystemType` / `SystemName` / `SystemVersion`、`ClientType` / `ClientName` / `ClientVersion`：八项允许继承的客户端环境字段，不表达可信身份。
- `AcceptLanguage`：只在入站缺少 Firefly 语言头时归一为语言偏好，不直接透传。
- `TraceParent` / `TraceState` / `Baggage`：标准 OTel/W3C 传播头。每次出站从当前活动 span/context 注入，不复制入口 trace 快照。
- `XRealIp` / `XForwardedFor`：受控入口适配所需的代理输入；必须先验证连接对端及代理可信性，不进入通用上下文入站/出站允许列表。
- `RequestMethod*` / `RequestMethod*String`：HTTP/gRPC 方法枚举。签名中的授权 `Method / Path` 与本地实际 `ServiceMethod / ServicePath` 分别表达，不互相回填。
- `SubjectTypeAnonymous` / `SubjectTypeUser` / `SubjectTypeService`：匿名、用户、服务主体类型。服务主体可以独立保留四字段用户上下文。
- `JWSAlgorithmEdDSA` / `JWSTypeJWT` / `AuthzSignDefault*` / `AuthzSignDecisionAllow` / `AuthzSignClaim*`：JOSE、默认 kid/issuer 及新签名 claim 的共享常量。
- `ExtAuthzContext*`：路由组件传给 ext_authz 的 `context_extensions` key；其中 `route_method / route_path` 与 `target_method / target_path` 仍是组件适配协议，不是普通业务请求头。

## 签名与本地身份

授权主体应用使用 `subject_app_id`，上一跳已认证服务保留 `invoke_service_app_id`；运行时 `InvokeAppId` 只能由后者派生，不接受普通头自报。

授权 API 使用 `method / path`；实际投递绑定使用 `service_app_id / service_method / service_path`。本地 `ServiceAppId / ServiceInstanceId` 来自配置，实际入口来自 HTTP/gRPC 适配器，不使用签名或请求头反填实际值，也不生成对应的本地身份 metadata。

公开业务视图保持 `service.Context`，其中用户视图仅含 `Session / UserId / AppId / TenantId`。ORM 由显式适配的可信 reader 读取最小身份，不解析原始 metadata、token 或签名。

## 出站允许列表

出站由 `service.OutgoingContext` 编码精确允许列表，`authz` 负责提供当前服务凭证及协议适配，不按 `x-firefly-*` 前缀整体放行：

- `x-firefly-user-authority`：框架私有继承的用户凭证，有值时发送。
- `x-firefly-service-authority`：当前服务 provider 提供的服务凭证，必须存在。
- `x-firefly-invoke-method` / `x-firefly-invoke-path`：当前服务实际入口，有值时发送。
- 八项客户端环境字段：有值时继承。
- `traceparent` / `tracestate` / `baggage`：当前活动 OTel 上下文注入的标准传播值。

不发送旧 AuthzSign、六项明文身份/授权观测头、Accept-Language、代理 IP、组件头、本地服务身份、身份集合、标准 Authorization 或未知 Firefly 头。HTTP 当次显式上传/协议头不属于通用上下文继承，不因本次改造删除。

## 已退出的旧协议

以下项目不再作为通用服务上下文读取、注入或透传；入口清洗旧值不等于兼容接受：

- `OrgIds / PostIds / RoleIds` 等身份集合及其请求头。
- 普通 `x-firefly-invoke-app-id / x-firefly-target-app-id`。
- 普通 `x-firefly-route-method / x-firefly-route-path / x-firefly-target-method / x-firefly-target-path`。
- `x-firefly-api-method / x-firefly-api-path / x-firefly-access-method`。
- `x-firefly-service-app-id / x-firefly-service-instance-id` 和 invoke/target instance 头。
- 旧网关签名、自定义 trace/span、service-auth、resource-type/resource-path 头。
- `authorization` 作为 Firefly 业务身份入口，以及旧 authz-error-code/authz-error-reason/token-refresh-required 头。

组件层 `ExtAuthzContext*` 常量不因名称含 Route/Target 被删除；其边界与业务上下文头不同。组织、角色、岗位业务实体也不因身份集合退出而删除。
