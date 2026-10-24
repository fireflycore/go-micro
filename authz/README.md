# Authz

`authz` 包提供业务服务接入 Firefly 数据面授权结果的通用配置与工具。

先区分四层对象：

- 传输层：跨进程只传 HTTP header / gRPC metadata。
- authority：`X-Firefly-User-Authority` / `X-Firefly-Service-Authority`，只给 authz 校验和解析。
- 签名层：`x-firefly-authz-sign` 是 authz allow 后写入的 compact JWS。
- 进程内：`service.Context` 和 `service.AuthzSign` 是业务服务本地结构体，不跨进程传输。

它负责：

- 定义服务侧验签配置结构体 `VerificationConfig`
- 加载 Ed25519 公钥 PEM
- 构造 `service.AuthzSignVerificationOptions`
- 返回 gRPC middleware 需要的 `AuthzVerification` 与 `AuthzSkipMethods`
- 定义 `ServiceAuthorityProvider` / ServiceToken 管理器，供出站调用每一跳覆盖 `X-Firefly-Service-Authority`
- 提供 unary / stream gRPC client interceptor、HTTP `AuthorityTransport` 和 metadata helper，独立构建当次出站白名单

`x-firefly-authz-sign` 只用于本跳入口验签，不向下游传播。业务上下文保存 `AuthzSignContext` 投影，不暴露原始签名或凭证。

用户应用读取 `service.Context.UserContext.AppId`；上一跳服务读取 `InvokeAppId`，只能来自已验签的 `invoke_service_app_id`。授权主体读取 `AuthzSignContext.SubjectAppId`。本地 `ServiceAppId / ServiceInstanceId` 不接受上游覆盖。

业务服务只需要在启动配置中声明：

```json
{
  "authz_verification": {
    "kid": "default",
    "public_key_path": "/etc/firefly/authz/keys/default-pub.pem",
    "issuer": "firefly-authz",
    "clock_skew": "5s",
    "skip_methods": [
      "/grpc.health.v1.Health/Check",
      "/grpc.health.v1.Health/Watch"
    ]
  }
}
```

业务入口必须装配 `authz_verification`，配置必须提供 `public_key_path`；HTTP/gRPC 适配器漏配 verifier 时拒绝执行 handler。不再通过 `enabled` 字段做运行时开关；健康、管理等独立入口只能通过精确 skip 配置跳过，跳过不建立可信身份。

启用验签时，入口必须传入本地 `ServiceAppId`；HTTP / gRPC 适配器独立捕获实际入口并校验签名 `service_app_id / service_method / service_path`。授权 `method / path` 不作为缺失投递绑定的回退。

gRPC 普通 metadata 仅支持可打印 ASCII，出站 helper 在传输前拒绝不能编码的值；HTTP `AuthorityTransport` 保持合法 UTF-8，不套用 gRPC 限制。未引入隐式转义或 `-bin` 头协议。

然后在 gRPC server 初始化时接入：

```go
verification, err := authz.NewVerificationOptions(bootstrapConfig.AuthzVerification)
if err != nil {
    panic(err)
}

gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
    ServiceAppId:      bootstrapConfig.App.Id,
    ServiceInstanceId: bootstrapConfig.App.InstanceId,
    AuthzVerification: verification.AuthzVerification,
    AuthzSkipMethods:  verification.AuthzSkipMethods,
})
```

当前只支持 EdDSA/Ed25519 JWS。`kid` 和 `issuer` 的默认值属于 `authz` 包内部配置语义，不放入 `constant` 公共常量。

## Service Authority

服务间调用需要同时遵守两条规则：

- 入站存在 `X-Firefly-User-Authority` 时继续透传，保持用户身份上下文。
- 每一跳由当前服务覆盖 `X-Firefly-Service-Authority`，表达当前调用方服务身份。

推荐在启动期把 auth 服务的 `GenerateServiceToken` 封装为 fetch 函数，并启动后台刷新：

```go
provider, err := authz.NewServiceAuthorityProvider(
    &authz.ServiceAuthorityConfig{
        RefreshBefore: "1m",
    },
    func(ctx context.Context) (*authz.ServiceAuthorityToken, error) {
        resp, err := authTokenClient.GenerateServiceToken(ctx, req)
        if err != nil {
            return nil, err
        }
        return authz.NewServiceAuthorityToken(resp.Data.Token, resp.Data.Expired)
    },
)
if err != nil {
    panic(err)
}
if err := provider.Start(ctx); err != nil {
    panic(err)
}
defer provider.Stop()

invoker := invocation.NewUnaryInvoker(manager, timeout).
    WithServiceAuthorityProvider(provider)
```

`ServiceAuthorityProvider` 会在进程内缓存 service token，并在后台按 `RefreshBefore` 主动刷新。首次 fetch 会在 `Start(ctx)` 后立即异步执行；失败后按 `min(1 minute * retry_count * 10, 60 minutes)` 退避并无限次重试，成功后清零。没有有效 service token 时，出站 Firefly 服务调用返回 `ErrServiceTokenUnavailable`，不会只携带用户 token 穿透下游。

管理端或排障接口可通过 `ServiceAuthorityStatus()` 读取安全状态快照。该快照只包含 `started`、`loaded`、`expires_at`、`expires_in_seconds`、`refresh_before` 和 `last_error`，不包含 service token 原文，适合挂到业务服务 `/info` 这类自管理接口。

出站只继承框架私有用户 authority 和八个客户端环境字段，覆盖当前服务 authority 与当前实际入口的 Invoke 方法，使用活动 OTel span 注入 trace / baggage。不发送旧签名、普通身份、组件 IP、本地身份或未知 Firefly 头；Accept-Language 不透传。provider 缺失、空值或失败均拒绝业务调用。

凭证获取使用独立、绑定单个方法的 `invocation.NewBootstrapUnaryInvoker`，不通过缺省 provider 豁免业务调用。下一跳 authz 必须验证所有携带凭证及各自在线 session，选择本跳主体后重新签发 60 秒证明；本库不实现 authz 的在线认证或签发。
