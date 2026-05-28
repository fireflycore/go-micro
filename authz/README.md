# Authz

`authz` 包提供业务服务接入 Firefly 数据面授权上下文的通用配置与工具。

它负责：

- 定义服务侧验签配置结构体 `VerificationConfig`
- 加载 Ed25519 公钥 PEM
- 构造 `service.AuthzContextVerificationOptions`
- 返回 gRPC middleware 需要的 `AuthzVerification` 与 `AuthzSkipMethods`

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
