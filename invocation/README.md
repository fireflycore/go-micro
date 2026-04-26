# Invocation

`invocation` 包定义 Firefly 当前唯一推荐的出站服务调用模型。

它只负责四件事：

- 用 `service.DNS` 表达远程业务服务
- 把 DNS 组装成稳定的 gRPC target
- 复用 `grpc.ClientConn`
- 统一构造出站 metadata 与 timeout

它不负责运行时治理能力：

- 实例发现
- 节点选择
- Consul / K8s 后端适配
- endpoint 轮询

## 必读顺序

- `README.md`：快速理解模型与推荐主线
- `ARCHITECTURE.md`：看清组件职责、边界和完整调用时序
- `USAGE.md`：看实际装配方式、repo 接入模式和示例
- `CONTEXT-USAGE.md`：查看旧上下文模型废弃说明
- `TEST_REPORT.md`：查看测试、覆盖率和基准结果

## 一句话主线

当前推荐主线只有一条：

1. 启动装配层创建 `DNSManager`
2. 启动装配层创建 `ConnectionManager`
3. 启动装配层创建 `UnaryInvoker`
4. 启动装配层创建 `RemoteServiceManaged`
5. repo 在 `New*Repo(...)` 中通过 `services.Caller("service")` 绑定 `RemoteServiceCaller`
6. repo 方法只保留 `full method + req + resp`

如果 repo 只依赖一个远程业务服务，也可以直接装配 `RemoteServiceCaller`。

## 核心对象

### `service.DNS`

远程业务服务的标准 DNS 描述。

推荐直接使用字面量，不再额外包一层 builder 或 option helper。

### `DNSManager`

负责默认值补齐、最小校验和最终 `Target` 构造。

### `ConnectionManager`

负责按最终 gRPC target 复用连接。

### `UnaryInvoker`

负责底层真实调用：

- 复用当前链路 metadata
- 注入 `ServiceAppId` / `ServiceInstanceId`
- 使用初始化时注入的统一 timeout
- 发起真实 gRPC unary 调用

### `RemoteServiceCaller`

repo 级远程业务服务调用入口。

适合“当前 repo 已经明确绑定一个远程业务服务”的场景。

### `RemoteServiceManaged`

多业务服务注册表。

适合“当前服务依赖多个远程业务服务”的场景，用来：

- 统一登记多组 `service.DNS`
- 统一复用一个 `UnaryInvoker`
- 按服务名派生 `RemoteServiceCaller`

## 推荐接入方式

推荐分成两层装配：

- 启动装配层：维护“本服务依赖哪些远程业务服务”
- repo 层：维护“当前 repo 绑定哪个远程业务服务 caller”

建议目录分工：

- provider / bootstrap / `internal/dep`：创建 `RemoteServiceManaged`
- `internal/data/rs_*.go`：在 `New*Repo(...)` 中绑定 `RemoteServiceCaller`
- repo 方法：只保留 `full method + req + resp`

完整示例见 `USAGE.md`。

## 关键约束

- 一个远程业务服务只维护一份 `service.DNS`
- 同一业务服务下多个 proto 子服务共用同一份 DNS 和连接
- 具体 RPC 由 gRPC `full method` 决定
- `UnaryInvoker` 不再作为 repo 层首选装配入口
- 不再暴露 metadata / timeout 的单次调用覆盖能力

## 快速示例

```go
package example

import (
	"time"

	"github.com/fireflycore/go-micro/invocation"
	"github.com/fireflycore/go-micro/service"
)

func BuildRemoteServices(manager *invocation.ConnectionManager) *invocation.RemoteServiceManaged {
	invoker := invocation.NewUnaryInvoker(manager, "config", "config-1", 3*time.Second)

	return invocation.NewRemoteServiceManaged(
		invoker,
		service.DNS{Service: "auth"},
		service.DNS{Service: "iam"},
	)
}
```

## 从哪里继续看

- 想看职责边界和完整时序：`ARCHITECTURE.md`
- 想看 repo 怎么接入：`USAGE.md`
- 想确认旧上下文为什么废弃：`CONTEXT-USAGE.md`
- 想看测试与性能现状：`TEST_REPORT.md`
