# Service

`service` 包定义服务内统一主上下文模型。

它只负责：

- 定义 `service.Context`
- 提供 `WithContext(...)` / `FromContext(...)` / `MustFromContext(...)`
- 提供 `BuildContext(...)` 把入站 metadata 与当前 OTel span 结构化为服务内主上下文
- 提供 `VerifyAuthzContext(...)` / `BuildVerifiedContext(...)` 对 authz 签名上下文本地验签

它不负责：

- 远程业务服务 DNS 建模
- gRPC interceptor 装配
- 出站 metadata 拼装
- 下游服务调用
- token 解析或在线权限判断

当前推荐分工：

- `middleware/grpc`：在 gRPC 服务端入口创建并注入 `service.Context`
- `service`：供 `service / biz / data / log` 统一读取服务内主上下文，并提供 authz JWS 本地验签能力
- `invocation`：负责远程业务服务 DNS 与纯出站调用语义

`ServiceContext` 中的 `AppId` 是调用方应用 ID 的业务侧兼容字段，新代码应优先使用语义更明确的 `InvokeAppId`；被访问资源所属应用 ID 使用 `TargetAppId`。
