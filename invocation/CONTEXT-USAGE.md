# Invocation Context Usage

本文件已废弃。

旧版 `UserContextMeta`、`WithUserContext(...)`、`UserContextFromContext(...)`、`MustUserContextFromContext(...)` 等用法已经从当前 `go-micro` 主路径中移除，不应继续作为服务内上下文模型使用。

## 当前边界

- 服务端入站 metadata 的解析与服务内主上下文建立，统一由 `middleware/grpc` 负责
- 服务内代码只读取 `ServiceContext`
- 服务调用侧只复用当前链路 metadata，并使用 `UnaryInvoker` 初始化时注入的统一 timeout

## 推荐入口

- `gm.NewServiceContextUnaryInterceptor(...)`
- `service.FromContext(...)`
- `invocation.NewRemoteServiceCaller(...)`
- `invocation.NewUnaryInvoker(...)`

## 参考文档

- `invocation/README.md`
- `middleware/grpc/README.md`
- `design/registry/current/invocation-context-boundary-plan.md`
