# Firefly Go Micro

`go-micro` 是 Firefly 微服务框架的 Go 版本核心库，提供了构建微服务所需的基础设施抽象与通用工具。

建议配合 **go-layout**（Firefly 微服务框架的 Go 版本标准项目模板）使用，以获得最佳开发体验。

当前版本同时处于两条能力线并行阶段：

- 旧能力线：`registry`，延续服务注册与节点发现模型
- 新能力线：`invocation`，面向 `service -> service` 的统一调用模型

长期方向以 `invocation` 为主，`registry` 将在迁移完成后退出主路径。

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
		gm.ValidationErrorToInvalidArgument(),
		gm.NewAccessLogger(log),
	),
)
_ = s
```

如果要面向新的服务调用模型，建议优先使用 `invocation` 包提供的能力：

- 用 `ServiceRef` 表达“我要调用哪个服务”
- 用 `Locator` 把服务身份解析成目标地址
- 用 `ConnectionManager` 统一管理 `grpc.ClientConn`
- 用 `Invoker` 统一串起 metadata、Authz 与底层调用
- 并默认把调用链路接入 OTel 观测体系

适用场景：

- `K8s + Istio` 标准主路径
- `etcd / consul` 轻量实现路径
- 需要统一调用入口而不是直接操作节点列表的场景

## 模块说明

详细文档请参考各子包目录下的 README：

- [invocation](./invocation/README.md)：新的服务调用模型（推荐）
- [registry](./registry/README.md)：服务发现与注册
- [rpc](./rpc/README.md)：RPC 调用封装
- [middleware](./middleware/README.md)：中间件（gRPC/HTTP）
- [logger](./logger/README.md)：zap/otelzap 日志封装
- [constant](./constant/README.md)：通用常量

## 当前建议

- 新项目优先围绕 `invocation` 设计服务间调用
- 旧项目若仍依赖 `registry`，可继续使用，但不建议再基于它扩展新模型
- `rpc` 包中的现有工具仍可复用，但长期应逐步向 `invocation` 收敛
- 新的调用能力默认应与 `telemetry`、`middleware/grpc` 中现有的 OTel 能力保持一致

## 许可证

[LICENSE](./LICENSE)
