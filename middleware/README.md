# Middleware

`middleware` 目录包含了 firefly 微服务框架的各类中间件组件。

## 目录结构

- **[gRPC Middleware (gm)](./grpc/README.md)**: `middleware/grpc`
  - 提供 gRPC 服务端的拦截器，包括访问日志、元数据透传、服务身份注入等功能。

- **HTTP Middleware (hm)**: (待添加)
  - *注：目前尚未包含 HTTP 中间件实现。*

## 快速导航

### gRPC Middleware (`gm`)

位于 `middleware/grpc` 包下。

主要功能：
- `NewServiceAccessLogger`: 详细的请求/响应日志与链路追踪信息记录。
- `PropagateIncomingMetadata`: 微服务链路元数据透传。
- `NewInjectServiceContext`: 服务自身身份信息注入。

详细文档请参考：[grpc/README.md](./grpc/README.md)
