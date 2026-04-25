# Firefly Go Micro

`go-micro` 是 Firefly 微服务框架的 Go 版本核心库，提供了构建微服务所需的基础设施抽象与通用工具。

建议配合 **go-layout**（Firefly 微服务框架的 Go 版本标准项目模板）使用，以获得最佳开发体验。

当前版本已经收敛为两条明确路径：

- 裸机接入路径：`go-consul/agent`，负责对接本机 `sidecar-agent`
- 服务调用路径：`invocation`，面向 `service -> service` 的统一调用模型

旧 `registry` 体系文档已经迁入 design 仓库的 `design/registry` 目录。

同时，`go-micro` 已明确采用 OpenTelemetry 作为统一观测体系：

- gRPC 调用链默认接入 OTel
- 指标、链路、日志相关能力围绕 OTel 生态组织
- `invocation` 的后续实现也应默认对齐这一观测模型

## 安装

```bash
go get github.com/fireflycore/go-micro
```

## 快速开始

以 gRPC 服务为例，常见用法是把中间件注入到 `grpc.Server`，并挂载 OpenTelemetry 的 gRPC StatsHandler：

```go
import (
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常为 gm
	"google.golang.org/grpc"
)

s := grpc.NewServer(
	grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
	grpc.ChainUnaryInterceptor(
		gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
			ServiceAppId:      "auth",
			ServiceInstanceId: "auth-1",
		}),
		gm.ValidationErrorToInvalidArgument(),
		gm.NewAccessLogger(log),
	),
)
_ = s
```

如果要面向新的服务调用模型，建议优先使用 `invocation` 包提供的能力：

- 用 `ServiceDNS` 表达“我要调用哪个业务服务 DNS”
- 用 `DNSManager` 统一组装标准 gRPC target
- 用 `ConnectionManager` 统一管理 `grpc.ClientConn`
- 用 `Invoker` 统一串起 metadata、Authz 与底层调用
- 并默认把调用链路接入 OTel 观测体系

适用场景：

- `go-k8s + K8s + Istio` 标准主路径
- `go-consul + consul + envoy + sidecar-agent` 裸机主路径
- 需要统一调用入口而不是直接操作节点列表的场景

## 模块说明

详细文档请参考各子包目录下的 README：

- [invocation](./invocation/README.md)：新的服务调用模型（推荐）
- [go-consul/agent](file:///Users/lhdht/product/firefly/go-consul/agent/README.md)：业务服务与本机 sidecar-agent 的联动桥接
- [middleware](./middleware/README.md)：中间件（gRPC/HTTP）
- [logger](./logger/README.md)：zap/otelzap 日志封装
- [constant](./constant/README.md)：通用常量

## 当前建议

- 新项目优先围绕 `invocation` 设计服务间调用
- 裸机业务服务统一通过 `go-consul/agent` 接入 sidecar-agent
- 新增的客户端接入统一通过 `invocation` 提供的连接、metadata 与调用能力落地
- 新的调用能力默认应与 `telemetry`、`middleware/grpc` 中现有的 OTel 能力保持一致

## 许可证

[LICENSE](./LICENSE)
