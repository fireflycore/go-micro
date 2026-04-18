# Invocation

`invocation` 包定义 Firefly 当前唯一推荐的服务调用模型。

它只解决四件事：

- 业务侧如何声明一个远程业务服务的标准 DNS
- 如何把 DNS 组装成稳定的 gRPC target
- 如何复用 `grpc.ClientConn`
- 如何统一传递 metadata、调用者身份与 Authz 上下文

它**不再**负责：

- 实例发现
- 节点选择
- Consul / K8s 后端适配
- endpoint 轮询

## 核心理念

业务服务之间的调用，本质上就是面向一个稳定的业务服务 DNS。

例如：

```text
auth.default.svc.cluster.local:9090
```

含义如下：

- `auth`：业务服务名
- `default`：命名空间
- `svc`：Kubernetes Service 类型片段
- `cluster.local`：Cluster Domain
- `9090`：业务服务端口

业务代码只需要表达这份 DNS 结构。

后续流量如何命中实例：

- 裸机环境交给 `sidecar-agent`
- 云原生环境交给 `K8s + Istio + service mesh`

## 当前模型

### ServiceDNS

`ServiceDNS` 表示业务服务的标准 DNS 配置。

它直接描述：

- `service`
- `namespace`
- `service_type`
- `cluster_domain`
- `port`

### DNSManager

`DNSManager` 只负责补齐默认值并构造最终 `Target`。

它不会做：

- service 校验
- endpoint 拉取
- 实例选择

### ConnectionManager

`ConnectionManager` 负责：

- 基于 `ServiceDNS` 构造 `Target`
- 按最终 gRPC target 缓存连接
- 统一挂载 gRPC client dial options

### UnaryInvoker

`UnaryInvoker` 负责：

- 取连接
- 注入调用 metadata
- 接入 Authz
- 发起真实 gRPC unary 调用

### InvocationContext

`InvocationContext` 负责：

- 统一 metadata
- 调用者身份
- timeout
- trace 相关上下文

## 一个业务服务多个 proto 子服务

这是当前模型里的重要约束：

- 一个远程**业务服务**只维护一份 `ServiceDNS`
- 同一个业务服务下的多个 proto 子服务，共用同一份 DNS 和连接
- 具体调用哪个子服务，由 gRPC full method 决定

例如：

- 远程业务服务：`auth`
- proto 子服务：
  - `AuthAppService`
  - `AuthUserService`
  - `AuthPermissionService`

这些调用都应该共用：

```text
auth.default.svc.cluster.local:9090
```

## 推荐接入方式

业务服务应在自己的 `internal/data/rs_*.go` 中，按“远程业务服务”聚合配置。

推荐做法：

- 在 `New*Repo` 中声明该 repo 依赖哪个远程业务服务
- 为该远程业务服务组装一份 `ServiceDNS`
- 复用同一份 `ConnectionManager / UnaryInvoker`
- 通过不同 full method 区分具体 proto 子服务

不推荐做法：

- 按每个 proto 子服务单独维护一份远程地址配置
- 在调用侧做实例发现
- 在调用侧感知 Consul / K8s 细节

## 示例

```go
package example

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/invocation"
)

func Example() error {
	dnsManager := invocation.NewDNSManager(&invocation.DNSConfig{
		DefaultNamespace: "default",
		DefaultPort:      9090,
	})

	manager, err := invocation.NewConnectionManager(invocation.ConnectionManagerOptions{
		DNSManager: dnsManager,
	})
	if err != nil {
		return err
	}
	defer func() { _ = manager.Close() }()

	invoker := &invocation.UnaryInvoker{
		Dialer: manager,
	}

	service := &invocation.ServiceDNS{
		Service: "auth",
	}

	return invoker.Invoke(
		context.Background(),
		service,
		"/acme.auth.app.v1.AuthAppService/GetAppSecret",
		&struct{}{},
		&struct{}{},
		invocation.WithInvocationContext(&invocation.InvocationContext{
			Timeout: 3 * time.Second,
		}),
	)
}
```

## 观测约定

`invocation` 默认和 `go-micro` 的 OTel 链路保持一致：

- gRPC client 默认挂 `otelgrpc`
- metadata 由 `InvocationContext` 统一构造
- Authz 上下文由 `NewAuthzContext()` 统一生成

## 设计约束

- 业务侧只表达业务服务 DNS，不表达实例选择逻辑
- `invocation` 只保留通用调用语义，不承载后端专属实现
- `go-consul/invocation`、`go-k8s/invocation` 不再作为主路径保留
- `Authz` 默认作为调用前外挂能力接入
