# Authz

`authz` 包提供业务服务接入 Firefly 数据面授权上下文的通用配置与工具。

它负责：

- 定义服务侧验签配置结构体 `VerificationConfig`
- 加载 Ed25519 公钥 PEM
- 构造 `service.AuthzContextVerificationOptions`
- 返回 gRPC middleware 需要的 `AuthzVerification` 与 `AuthzSkipMethods`
- 定义 `ServiceAuthorityProvider`，供出站调用每一跳覆盖 `X-Firefly-Service-Authority`
- 提供 gRPC client interceptor 和 metadata helper，统一清理上一跳 authz 上下文

业务服务只需要在启动配置中声明：

```json
{
  "authz_verification": {
    "enabled": true,
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

然后在 gRPC server 初始化时接入：

```go
verification, err := authz.NewVerificationOptions(bootstrapConfig.AuthzVerification)
if err != nil {
    panic(err)
}

gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
    ServiceAppId:        bootstrapConfig.App.Id,
    ServiceInstanceId:   bootstrapConfig.App.InstanceId,
    AuthzVerification:   verification.AuthzVerification,
    AuthzSkipMethods:    verification.AuthzSkipMethods,
})
```

当前只支持 EdDSA/Ed25519 JWS，`kid` 默认固定为 `default`。

## Service Authority

服务间调用需要同时遵守两条规则：

- 入站存在 `X-Firefly-User-Authority` 时继续透传，保持用户身份上下文。
- 每一跳由当前服务覆盖 `X-Firefly-Service-Authority`，表达当前调用方服务身份。

推荐在启动期把 auth 服务的 `GenerateServiceToken` 封装为 fetch 函数：

```go
provider, err := authz.NewServiceAuthorityProvider(
    &authz.ServiceAuthorityConfig{
        Enabled:       true,
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

invoker := invocation.NewUnaryInvoker(manager, appID, instanceID, timeout).
    WithServiceAuthorityProvider(provider)
```

`ServiceAuthorityProvider` 会在进程内缓存 service token，并在过期前按 `RefreshBefore` 主动刷新。出站 metadata 会保留用户 authority 和 OTel trace 头，但会移除上一跳的 `Authorization`、`x-firefly-authz-context`、普通用户上下文字段和授权资源字段，避免把上一跳的授权结果错误复用到下一跳。
