# Invocation Context Usage

本文件保留为兼容入口，旧上下文模型已经废弃。

旧版 `UserContextMeta`、`WithUserContext(...)`、`UserContextFromContext(...)`、`MustUserContextFromContext(...)` 等做法已经从当前 `go-micro` 主路径移除，不应继续作为服务内上下文模型使用。

## 当前边界

- 服务端入站 metadata 的解析与服务内主上下文建立，统一由 `middleware/grpc` 负责
- 服务内代码只读取 `ServiceContext`
- 出站调用只复用当前链路 metadata，并使用 `UnaryInvoker` 初始化时注入的统一 timeout

## 当前推荐入口

- `gm.NewServiceContextUnaryInterceptor(...)`
- `service.FromContext(...)`
- `invocation.NewUnaryInvoker(...)`
- `invocation.NewRemoteServiceManaged(...)`
- `services.Caller("service")`

## 当前推荐装配

- 在服务启动装配层集中创建 `invocation.NewRemoteServiceManaged(...)`
- 在各自 repo 的 `New*Repo(...)` 中通过 `services.Caller("service")` 绑定 `RemoteServiceCaller`
- 若项目已有 `internal/dep`、provider 或 bootstrap 层，多业务服务注册表优先放在那里

## 参考文档

- `README.md`
- `ARCHITECTURE.md`
- `USAGE.md`
- `middleware/grpc/README.md`
- `design/registry/current/invocation-context-boundary-plan.md`
