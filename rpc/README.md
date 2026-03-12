# RPC

`rpc` 包提供了标准化的 RPC 调用封装与响应处理工具。

## 设计理念

本包聚焦两件事：
- 统一创建“服务调用下游服务”的出站 `context.Context`（metadata 透传、服务身份注入、超时控制）。
- 统一构造 gRPC Client 连接选项（默认启用 OpenTelemetry gRPC client stats handler）。

## 常用入口

### 创建 gRPC ClientConn

`NewGrpcClient` 默认对连接启用 `otelgrpc.NewClientHandler()`，用于自动注入/提取 W3C `traceparent`（链路追踪）与采集指标。

### 构造出站 Context

`ServiceContext` 用于构建服务级别静态 metadata，并提供三种出站 ctx 构造方法：
- `WithPureContext`：纯净出站 ctx（不继承请求 metadata），适合定时任务/后台任务。
- `WithExternalContext`：基于外部传入 md 合并服务静态 metadata，适合消息队列/Webhook 等入口。
- `WithInheritContext`：继承 parent 的 metadata 与 span context，再合并服务静态 metadata，适合处理请求时继续调用下游服务。
