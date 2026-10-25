# Service

本地实现链路为 `IncomingContext → Context → OutgoingContext`。运行时仍是
`service.Context`，标识为 `service.context`，不把三个对象当成同一结构或复制原始 metadata。

## 入口与业务视图

- `CollectIncomingContext` 收集逐项允许的输入，收集不代表认证。
- `NewIncomingContext` 本地验签并剥离原始凭证；用户 authority 仅留在框架私有 context key。
- `BuildContext` / `BuildVerifiedContext` 返回 `(*Context, error)`；不吞掉输入和验签错误。
- `WithContext` / `FromContext` 防御性复制运行时及嵌套视图。
- `UserContext` 仅含 `Session / UserId / AppId / TenantId`，AppId 是用户登录应用。
- `AuthzSignContext` 只有已验证授权事实；业务 Context 不包含 authority 或 JWS 原文。
- `InvokeAppId` 只派生自签名 `invoke_service_app_id`，不取用户应用或明文头。

本地配置提供 `ServiceAppId / ServiceInstanceId`；HTTP / gRPC 适配器捕获实际
`ServiceMethod / ServicePath`。签名的 `service_app_id / service_method / service_path`
必须与它们匹配，不能反填实际入口。授权 `method / path` 可以与投递入口不同。

`VerifyAuthzSign` 只支持 Ed25519、allow、固定 60 秒有效期和最多 5 秒时钟偏差，
拒绝旧 claim、未知字段、重复字段及缺失绑定。user 主体应用须等于用户 AppId；
service 主体应用须等于已认证上一跳服务，且可同时保留 UserContext。
业务服务不在线重查身份 session，该责任仍属于 authz。

## 必需用户视图

需要用户身份的用例使用 `RequireUserFromContext`，集中处理无用户场景：

```go
user, err := service.RequireUserFromContext(ctx)
if err != nil {
    return err
}

row.UserId = user.UserId
row.AppId = user.AppId
row.TenantId = user.TenantId
```

函数只读取已验证视图并返回副本，不解析原始 token/metadata，也不再次查询在线 session。
匿名或纯服务请求没有用户时返回 `werror.Unauthenticated`，gRPC 出口识别为 `Unauthenticated`；
服务主体同时携带用户时正常返回，不要求授权主体为 user。缺少 `service.Context` 仍沿用
`MustFromContext` 暴露装配错误，不补空用户或默认上下文。
允许无用户的接口继续通过 `FromContext` / `MustFromContext` 获取运行时视图，并按自身业务契约
处理可选的 `UserContext`；不安装全局“必须有用户”规则，也不统一改变空列表等既有语义。

## 出站与日志

`BuildOutgoingContext` 为单次 `OutgoingTarget{TargetAppId, TargetMethod, TargetPath}` 建立独立视图。
`authz.PrepareOutgoingAuthorityMetadata` 必须从当前 provider 获取服务凭证，用户凭证走私有继承；
不继承上一跳 AuthzSign。Invoke 方法取当前实际入口，只用于观测。
OTel 从当前活动 context 注入，取消和期限继续沿原生 context 传播。

入站标量拒绝重复值、非法 UTF-8 和控制字符；InvokeMethod 最长 32 UTF-8 字节，
InvokePath 最长 2048 字节。其余普通字段 2048 字节，authority / JWS / baggage 16384 字节。
Accept-Language 仅在 Firefly 语言头缺失时归一，不通用透传。
gRPC 出站适配另检查普通 metadata 的可打印 ASCII 限制，不能编码的值提前报错；
HTTP 保持合法 UTF-8，不私自增加转义或二进制头协议。

`TrustedLogIdentity` 只投影 UserId / AppId / TenantId / ServiceAppId / ServiceInstanceId，
不含 session、凭证或请求入口，供 ORM 接入层适配。请求入口和 Invoke 信息属于访问日志，
不要求每条 SQL 日志重复记录。
普通业务日志不自动附加整份请求上下文，仍只补充既有 trace/span 与日志类型；访问日志显式提取请求归因。
默认访问日志只读取可信结构化身份与本地服务事实，不读取原始 metadata、body 或错误原文。
健康、管理、认证启动及 ext_authz 入口须显式配置独立契约；跳过验签不建立可信用户身份。
HTTP/gRPC 业务适配器漏装验签配置时拒绝执行 handler；公共接口的匿名访问仍需 authz 签发匿名证明，不通过漏配配置实现。

此协议是破坏性变更，无旧字段别名或双读。核心库发布后仍须协调 authz 与全部在线消费者切换；
不能把本地框架测试当作线上链路验收。

